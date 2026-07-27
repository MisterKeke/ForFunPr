package backend

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const favoriteNewsRetention = 30 * 24 * time.Hour

type appStateStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type appState struct {
	PreviousOpenedAt  string `json:"previous_opened_at"`
	CurrentOpenedAt   string `json:"current_opened_at"`
	PreviousRefreshAt string `json:"previous_refresh_at"`
	LastRefreshAt     string `json:"last_refresh_at"`
}

type UpdateWindow struct {
	PublishedAfter string `json:"published_after"`
	PublishedUntil string `json:"published_until"`
}

type UpdateWindows struct {
	NewWhileClosed UpdateWindow `json:"new_while_closed"`
	NewWhileOpen   UpdateWindow `json:"new_while_open"`
}

type FavoriteUpdateState struct {
	PreviousOpenedAt  string        `json:"previous_opened_at"`
	CurrentOpenedAt   string        `json:"current_opened_at"`
	PreviousRefreshAt string        `json:"previous_refresh_at"`
	LastRefreshAt     string        `json:"last_refresh_at"`
	UpdateWindows     UpdateWindows `json:"update_windows"`
}

func (a *Service) RecordAppOpen() error {
	ctx := a.requestContext()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin app-open transaction: %w", err)
	}
	defer tx.Rollback()

	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339)
	oldCurrentOpenedAt := now

	err = tx.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, "current_opened_at").Scan(&oldCurrentOpenedAt)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err := upsertAppStateValue(ctx, tx, "previous_opened_at", oldCurrentOpenedAt); err != nil {
		return err
	}
	if err := upsertAppStateValue(ctx, tx, "current_opened_at", now); err != nil {
		return err
	}
	if err := insertAppStateValueIfMissing(ctx, tx, "previous_refresh_at", now); err != nil {
		return err
	}
	if err := insertAppStateValueIfMissing(ctx, tx, "last_refresh_at", now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM favorite_news_items
		WHERE discovered_at < ?
	`, nowTime.Add(-favoriteNewsRetention).Format(time.RFC3339)); err != nil {
		return fmt.Errorf("prune expired favorite news: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM favorite_news_state WHERE id = 1`); err != nil {
		return fmt.Errorf("reset favorite news scan state: %w", err)
	}
	return tx.Commit()
}

func (a *Service) GetFavoriteUpdateState() (FavoriteUpdateState, error) {
	state, err := a.getAppState()
	if err != nil {
		return FavoriteUpdateState{}, err
	}
	return favoriteUpdateStateFromAppState(state), nil
}

func favoriteUpdateStateFromAppState(state appState) FavoriteUpdateState {
	return FavoriteUpdateState{
		PreviousOpenedAt:  state.PreviousOpenedAt,
		CurrentOpenedAt:   state.CurrentOpenedAt,
		PreviousRefreshAt: state.PreviousRefreshAt,
		LastRefreshAt:     state.LastRefreshAt,
		UpdateWindows:     buildUpdateWindows(state),
	}
}

func (a *Service) GetUpdateWindows() (UpdateWindows, error) {
	state, err := a.getAppState()
	if err != nil {
		return UpdateWindows{}, err
	}
	return buildUpdateWindows(state), nil
}

func (a *Service) getAppState() (appState, error) {
	return getAppState(a.requestContext(), a.db)
}

func getAppState(ctx context.Context, store appStateStore) (appState, error) {
	values := map[string]string{}
	rows, err := store.QueryContext(ctx, `
		SELECT key, value
		FROM app_state
		WHERE key IN (
			'previous_opened_at',
			'current_opened_at',
			'previous_refresh_at',
			'last_refresh_at'
		)
	`)
	if err != nil {
		return appState{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return appState{}, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return appState{}, err
	}

	return appState{
		PreviousOpenedAt:  values["previous_opened_at"],
		CurrentOpenedAt:   values["current_opened_at"],
		PreviousRefreshAt: values["previous_refresh_at"],
		LastRefreshAt:     values["last_refresh_at"],
	}, nil
}

func buildUpdateWindows(state appState) UpdateWindows {
	return UpdateWindows{
		NewWhileClosed: UpdateWindow{
			PublishedAfter: state.PreviousOpenedAt,
			PublishedUntil: state.CurrentOpenedAt,
		},
		NewWhileOpen: UpdateWindow{
			PublishedAfter: state.PreviousRefreshAt,
			PublishedUntil: state.LastRefreshAt,
		},
	}
}

func upsertAppStateValue(ctx context.Context, store appStateStore, key string, value string) error {
	_, err := store.ExecContext(ctx, `
		INSERT INTO app_state (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func insertAppStateValueIfMissing(ctx context.Context, store appStateStore, key string, value string) error {
	_, err := store.ExecContext(ctx, `
		INSERT OR IGNORE INTO app_state (key, value)
		VALUES (?, ?)
	`, key, value)
	return err
}
