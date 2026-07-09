package backend

import (
	"database/sql"
	"time"
)

type AppState struct {
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

func (a *App) ensureAppStateTable() error {
	_, err := a.db.Exec(`
		CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	return err
}

func (a *App) RecordAppOpen() error {
	now := time.Now().UTC().Format(time.RFC3339)
	oldCurrentOpenedAt := now

	err := a.db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, "current_opened_at").Scan(&oldCurrentOpenedAt)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err := a.upsertAppStateValue("previous_opened_at", oldCurrentOpenedAt); err != nil {
		return err
	}
	if err := a.upsertAppStateValue("current_opened_at", now); err != nil {
		return err
	}
	if err := a.insertAppStateValueIfMissing("previous_refresh_at", now); err != nil {
		return err
	}
	return a.insertAppStateValueIfMissing("last_refresh_at", now)
}

func (a *App) GetFavoriteUpdateState() (FavoriteUpdateState, error) {
	state, err := a.getAppState()
	if err != nil {
		return FavoriteUpdateState{}, err
	}

	windows := buildUpdateWindows(state)
	return FavoriteUpdateState{
		PreviousOpenedAt:  state.PreviousOpenedAt,
		CurrentOpenedAt:   state.CurrentOpenedAt,
		PreviousRefreshAt: state.PreviousRefreshAt,
		LastRefreshAt:     state.LastRefreshAt,
		UpdateWindows:     windows,
	}, nil
}

func (a *App) GetUpdateWindows() (UpdateWindows, error) {
	state, err := a.getAppState()
	if err != nil {
		return UpdateWindows{}, err
	}
	return buildUpdateWindows(state), nil
}

func (a *App) RecordRefresh() (FavoriteUpdateState, error) {
	state, err := a.getAppState()
	if err != nil {
		return FavoriteUpdateState{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	previousRefreshAt := state.LastRefreshAt
	if previousRefreshAt == "" {
		previousRefreshAt = state.CurrentOpenedAt
	}
	if previousRefreshAt == "" {
		previousRefreshAt = now
	}

	if err := a.upsertAppStateValue("previous_refresh_at", previousRefreshAt); err != nil {
		return FavoriteUpdateState{}, err
	}
	if err := a.upsertAppStateValue("last_refresh_at", now); err != nil {
		return FavoriteUpdateState{}, err
	}

	return a.GetFavoriteUpdateState()
}

func (a *App) getAppState() (AppState, error) {
	values := map[string]string{}
	rows, err := a.db.Query(`
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
		return AppState{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return AppState{}, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return AppState{}, err
	}

	return AppState{
		PreviousOpenedAt:  values["previous_opened_at"],
		CurrentOpenedAt:   values["current_opened_at"],
		PreviousRefreshAt: values["previous_refresh_at"],
		LastRefreshAt:     values["last_refresh_at"],
	}, nil
}

func buildUpdateWindows(state AppState) UpdateWindows {
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

func (a *App) upsertAppStateValue(key string, value string) error {
	_, err := a.db.Exec(`
		INSERT INTO app_state (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (a *App) insertAppStateValueIfMissing(key string, value string) error {
	_, err := a.db.Exec(`
		INSERT OR IGNORE INTO app_state (key, value)
		VALUES (?, ?)
	`, key, value)
	return err
}
