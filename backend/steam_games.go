package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	steamGameRefreshWorkerLimit   = 4
	maximumSteamRefreshErrorRunes = 1000
	steamGameResourceName         = "Steam game"
)

type SteamGame struct {
	ID         int    `json:"id"`
	SteamAppID uint32 `json:"steam_app_id"`

	StoreURL       string `json:"store_url"`
	Name           string `json:"name"`
	ImageURL       string `json:"image_url"`
	ImageSourceURL string `json:"image_source_url,omitempty"`

	PriceStatus string `json:"price_status"`
	Currency    string `json:"currency,omitempty"`

	RegularPriceMinor *int64 `json:"regular_price_minor"`
	CurrentPriceMinor *int64 `json:"current_price_minor"`
	DiscountPercent   int    `json:"discount_percent"`

	PriceCountryCode string `json:"price_country_code"`

	LastCheckedAt    string `json:"last_checked_at,omitempty"`
	LastAttemptedAt  string `json:"last_attempted_at,omitempty"`
	LastRefreshError string `json:"last_refresh_error,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type SteamGameSettings struct {
	CountryCode string `json:"country_code"`
	UpdatedAt   string `json:"updated_at"`
}

type SteamGameRefreshError struct {
	GameID     int    `json:"game_id"`
	SteamAppID uint32 `json:"steam_app_id"`
	Name       string `json:"name"`
	Error      string `json:"error"`
}

type SteamGameRefreshResult struct {
	Games []SteamGame `json:"games"`

	Attempted int `json:"attempted"`
	Updated   int `json:"updated"`
	Failed    int `json:"failed"`

	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`

	Errors []SteamGameRefreshError `json:"errors"`
}

type steamGameScanner interface {
	Scan(dest ...any) error
}

type steamGameQueryStore interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type steamGameRefreshFetch struct {
	Game        SteamGame
	Fetched     fetchedSteamGame
	AttemptedAt string
	Err         error
}

func (a *Service) ListSteamGamesContext(ctx context.Context) ([]SteamGame, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT
			games.id,
			games.steam_app_id,
			games.store_url,
			games.name,
			games.image_source_url,
			images.filename,
			games.price_status,
			games.currency,
			games.regular_price_minor,
			games.current_price_minor,
			games.discount_percent,
			games.price_country_code,
			games.last_checked_at,
			games.last_attempted_at,
			games.last_refresh_error,
			games.created_at,
			games.updated_at
		FROM steam_games AS games
		LEFT JOIN steam_game_images AS images ON images.game_id = games.id
		ORDER BY games.name COLLATE NOCASE ASC, games.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list Steam games: %w", err)
	}
	defer rows.Close()

	games := []SteamGame{}
	for rows.Next() {
		game, err := scanSteamGame(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Steam game: %w", err)
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Steam games: %w", err)
	}
	return games, nil
}

func loadSteamGameContext(
	ctx context.Context,
	store steamGameQueryStore,
	id int,
) (SteamGame, error) {
	if err := validateSteamGameID(id); err != nil {
		return SteamGame{}, err
	}
	game, err := scanSteamGame(store.QueryRowContext(ctx, `
		SELECT
			games.id,
			games.steam_app_id,
			games.store_url,
			games.name,
			games.image_source_url,
			images.filename,
			games.price_status,
			games.currency,
			games.regular_price_minor,
			games.current_price_minor,
			games.discount_percent,
			games.price_country_code,
			games.last_checked_at,
			games.last_attempted_at,
			games.last_refresh_error,
			games.created_at,
			games.updated_at
		FROM steam_games AS games
		LEFT JOIN steam_game_images AS images ON images.game_id = games.id
		WHERE games.id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return SteamGame{}, &NotFoundError{Resource: steamGameResourceName, Key: fmt.Sprint(id)}
	}
	if err != nil {
		return SteamGame{}, fmt.Errorf("load Steam game: %w", err)
	}
	return game, nil
}

func scanSteamGame(scanner steamGameScanner) (SteamGame, error) {
	var game SteamGame
	var steamAppID int64
	var imageFilename sql.NullString
	var currency sql.NullString
	var regularPrice sql.NullInt64
	var currentPrice sql.NullInt64
	var lastCheckedAt sql.NullString
	var lastAttemptedAt sql.NullString

	err := scanner.Scan(
		&game.ID,
		&steamAppID,
		&game.StoreURL,
		&game.Name,
		&game.ImageSourceURL,
		&imageFilename,
		&game.PriceStatus,
		&currency,
		&regularPrice,
		&currentPrice,
		&game.DiscountPercent,
		&game.PriceCountryCode,
		&lastCheckedAt,
		&lastAttemptedAt,
		&game.LastRefreshError,
		&game.CreatedAt,
		&game.UpdatedAt,
	)
	if err != nil {
		return SteamGame{}, err
	}
	if steamAppID <= 0 || steamAppID > int64(^uint32(0)) {
		return SteamGame{}, fmt.Errorf("stored Steam App ID is invalid")
	}
	game.SteamAppID = uint32(steamAppID)
	if imageFilename.Valid {
		game.ImageURL = steamGameImageURL(imageFilename.String)
	}
	if currency.Valid {
		game.Currency = currency.String
	}
	if regularPrice.Valid {
		value := regularPrice.Int64
		game.RegularPriceMinor = &value
	}
	if currentPrice.Valid {
		value := currentPrice.Int64
		game.CurrentPriceMinor = &value
	}
	if lastCheckedAt.Valid {
		game.LastCheckedAt = lastCheckedAt.String
	}
	if lastAttemptedAt.Valid {
		game.LastAttemptedAt = lastAttemptedAt.String
	}
	return game, nil
}

func (a *Service) GetSteamGameSettingsContext(ctx context.Context) (SteamGameSettings, error) {
	var settings SteamGameSettings
	err := a.db.QueryRowContext(ctx, `
		SELECT country_code, updated_at
		FROM steam_game_settings
		WHERE id = 1
	`).Scan(&settings.CountryCode, &settings.UpdatedAt)
	if err != nil {
		return SteamGameSettings{}, fmt.Errorf("read Steam game settings: %w", err)
	}
	return settings, nil
}

func (a *Service) SetSteamGameCountryContext(
	ctx context.Context,
	countryCode string,
) (SteamGameSettings, error) {
	a.steamGameRefreshMu.Lock()
	defer a.steamGameRefreshMu.Unlock()

	countryCode, err := normalizeSteamCountryCode(countryCode)
	if err != nil {
		return SteamGameSettings{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := a.db.ExecContext(ctx, `
		UPDATE steam_game_settings
		SET country_code = ?, updated_at = ?
		WHERE id = 1
	`, countryCode, now)
	if err != nil {
		return SteamGameSettings{}, fmt.Errorf("save Steam store country: %w", err)
	}
	if err := requireSingleMutation(result, "save Steam store country", "Steam game settings", false); err != nil {
		return SteamGameSettings{}, err
	}
	return a.GetSteamGameSettingsContext(ctx)
}

func (a *Service) AddSteamGameContext(ctx context.Context, storeURL string) (SteamGame, error) {
	steamAppID, canonicalURL, err := parseSteamGameURL(storeURL)
	if err != nil {
		return SteamGame{}, err
	}
	if err := ensureSteamGameAvailable(ctx, a.db, steamAppID); err != nil {
		return SteamGame{}, err
	}
	settings, err := a.GetSteamGameSettingsContext(ctx)
	if err != nil {
		return SteamGame{}, err
	}
	fetched, err := a.fetchSteamGameContext(ctx, steamAppID, settings.CountryCode)
	if err != nil {
		return SteamGame{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return SteamGame{}, fmt.Errorf("begin Steam game creation: %w", err)
	}
	defer tx.Rollback()
	if err := ensureSteamGameAvailable(ctx, tx, steamAppID); err != nil {
		return SteamGame{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO steam_games (
			steam_app_id,
			store_url,
			name,
			image_source_url,
			price_status,
			currency,
			regular_price_minor,
			current_price_minor,
			discount_percent,
			price_country_code,
			last_checked_at,
			last_attempted_at,
			last_refresh_error,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)
	`,
		int64(steamAppID),
		canonicalURL,
		fetched.Name,
		fetched.ImageSourceURL,
		fetched.PriceStatus,
		nullableSteamCurrency(fetched.Currency),
		nullableSteamPrice(fetched.RegularPriceMinor),
		nullableSteamPrice(fetched.CurrentPriceMinor),
		fetched.DiscountPercent,
		settings.CountryCode,
		now,
		now,
		now,
		now,
	)
	if err != nil {
		return SteamGame{}, fmt.Errorf("save Steam game: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return SteamGame{}, fmt.Errorf("read Steam game ID: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return SteamGame{}, fmt.Errorf("commit Steam game creation: %w", err)
	}

	gameID := int(id)
	if fetched.ImageSourceURL != "" {
		_ = a.cacheSteamGameImageContext(ctx, gameID, steamAppID, fetched.ImageSourceURL)
	}
	return loadSteamGameContext(ctx, a.db, gameID)
}

func ensureSteamGameAvailable(
	ctx context.Context,
	store steamGameQueryStore,
	steamAppID uint32,
) error {
	var existingID int
	err := store.QueryRowContext(ctx, `
		SELECT id
		FROM steam_games
		WHERE steam_app_id = ?
	`, int64(steamAppID)).Scan(&existingID)
	if err == nil {
		return &ConflictError{
			Resource: steamGameResourceName,
			Message:  "This Steam game is already being tracked.",
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check tracked Steam game: %w", err)
	}
	return nil
}

func (a *Service) DeleteSteamGameContext(ctx context.Context, id int) error {
	if err := validateSteamGameID(id); err != nil {
		return err
	}
	if _, err := loadSteamGameContext(ctx, a.db, id); err != nil {
		return err
	}

	var filename string
	imageErr := a.db.QueryRowContext(
		ctx,
		`SELECT filename FROM steam_game_images WHERE game_id = ?`,
		id,
	).Scan(&filename)
	if imageErr != nil && !errors.Is(imageErr, sql.ErrNoRows) {
		return fmt.Errorf("load Steam artwork before deletion: %w", imageErr)
	}

	var staged *stagedSteamGameImage
	var err error
	if imageErr == nil {
		staged, err = stageSteamGameImageDeletion(filename)
		if err != nil {
			return err
		}
	}
	restoreImage := func() {
		if staged != nil {
			staged.restore()
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		restoreImage()
		return fmt.Errorf("begin Steam game deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM steam_games WHERE id = ?`, id)
	if err != nil {
		restoreImage()
		return fmt.Errorf("delete Steam game: %w", err)
	}
	if err := requireSingleMutation(result, "delete Steam game", steamGameResourceName, false); err != nil {
		restoreImage()
		return err
	}
	if err := tx.Commit(); err != nil {
		restoreImage()
		return fmt.Errorf("commit Steam game deletion: %w", err)
	}
	if staged != nil {
		staged.finish()
	}
	return nil
}

func validateSteamGameID(id int) error {
	if id <= 0 {
		return &ValidationError{
			Field:   "id",
			Message: "Choose a valid tracked Steam game.",
		}
	}
	return nil
}

func nullableSteamCurrency(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableSteamPrice(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func (a *Service) RefreshSteamGamesContext(ctx context.Context) (SteamGameRefreshResult, error) {
	a.steamGameRefreshMu.Lock()
	defer a.steamGameRefreshMu.Unlock()
	return a.refreshSteamGamesLocked(ctx)
}

func (a *Service) RefreshSteamGamesOnOpenContext(ctx context.Context) (SteamGameRefreshResult, error) {
	a.steamGameRefreshMu.Lock()
	defer a.steamGameRefreshMu.Unlock()

	if a.steamInitialRefreshDone {
		result := a.steamInitialRefreshResult
		if games, err := a.ListSteamGamesContext(ctx); err == nil {
			result.Games = games
		}
		return result, a.steamInitialRefreshErr
	}

	a.steamInitialRefreshDone = true
	result, err := a.refreshSteamGamesLocked(ctx)
	a.steamInitialRefreshResult = result
	a.steamInitialRefreshErr = err
	return result, err
}

func (a *Service) refreshSteamGamesLocked(ctx context.Context) (SteamGameRefreshResult, error) {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	result := SteamGameRefreshResult{
		Games:     []SteamGame{},
		StartedAt: startedAt,
		Errors:    []SteamGameRefreshError{},
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	settings, err := a.GetSteamGameSettingsContext(ctx)
	if err != nil {
		return result, err
	}
	games, err := a.ListSteamGamesContext(ctx)
	if err != nil {
		return result, err
	}
	result.Attempted = len(games)
	if len(games) == 0 {
		result.Games = games
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		return result, nil
	}

	fetches := make([]steamGameRefreshFetch, len(games))
	runBounded(ctx, len(games), steamGameRefreshWorkerLimit, func(workerContext context.Context, index int) {
		game := games[index]
		attemptedAt := time.Now().UTC().Format(time.RFC3339)
		fetched, fetchErr := a.fetchSteamGameContext(
			workerContext,
			game.SteamAppID,
			settings.CountryCode,
		)
		fetches[index] = steamGameRefreshFetch{
			Game:        game,
			Fetched:     fetched,
			AttemptedAt: attemptedAt,
			Err:         fetchErr,
		}
	})
	if err := ctx.Err(); err != nil {
		return result, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin Steam game refresh: %w", err)
	}
	defer tx.Rollback()
	for _, fetch := range fetches {
		if fetch.Err != nil {
			errorMessage := safeSteamRefreshError(fetch.Err)
			updateResult, err := tx.ExecContext(ctx, `
				UPDATE steam_games
				SET
					last_attempted_at = ?,
					last_refresh_error = ?,
					updated_at = ?
				WHERE id = ?
			`, fetch.AttemptedAt, errorMessage, fetch.AttemptedAt, fetch.Game.ID)
			if err != nil {
				return result, fmt.Errorf("record Steam game refresh failure: %w", err)
			}
			if err := requireSingleMutation(updateResult, "record Steam game refresh failure", steamGameResourceName, false); err != nil {
				return result, err
			}
			result.Failed++
			result.Errors = append(result.Errors, SteamGameRefreshError{
				GameID:     fetch.Game.ID,
				SteamAppID: fetch.Game.SteamAppID,
				Name:       fetch.Game.Name,
				Error:      errorMessage,
			})
			continue
		}

		fetched := fetch.Fetched
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE steam_games
			SET
				name = ?,
				image_source_url = ?,
				price_status = ?,
				currency = ?,
				regular_price_minor = ?,
				current_price_minor = ?,
				discount_percent = ?,
				price_country_code = ?,
				last_checked_at = ?,
				last_attempted_at = ?,
				last_refresh_error = '',
				updated_at = ?
			WHERE id = ?
		`,
			fetched.Name,
			fetched.ImageSourceURL,
			fetched.PriceStatus,
			nullableSteamCurrency(fetched.Currency),
			nullableSteamPrice(fetched.RegularPriceMinor),
			nullableSteamPrice(fetched.CurrentPriceMinor),
			fetched.DiscountPercent,
			settings.CountryCode,
			fetch.AttemptedAt,
			fetch.AttemptedAt,
			fetch.AttemptedAt,
			fetch.Game.ID,
		)
		if err != nil {
			return result, fmt.Errorf("update refreshed Steam game: %w", err)
		}
		if err := requireSingleMutation(updateResult, "update refreshed Steam game", steamGameResourceName, false); err != nil {
			return result, err
		}
		result.Updated++
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit Steam game refresh: %w", err)
	}

	for _, fetch := range fetches {
		if fetch.Err != nil || fetch.Fetched.ImageSourceURL == "" {
			continue
		}
		if fetch.Game.ImageURL != "" && sameSteamArtworkAsset(
			fetch.Game.ImageSourceURL,
			fetch.Fetched.ImageSourceURL,
		) {
			continue
		}
		_ = a.cacheSteamGameImageContext(
			ctx,
			fetch.Game.ID,
			fetch.Game.SteamAppID,
			fetch.Fetched.ImageSourceURL,
		)
	}

	result.Games, err = a.ListSteamGamesContext(ctx)
	if err != nil {
		return result, err
	}
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func safeSteamRefreshError(err error) string {
	if err == nil {
		return "Steam game refresh failed."
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "Steam game refresh failed."
	}
	runes := []rune(message)
	if len(runes) > maximumSteamRefreshErrorRunes {
		message = string(runes[:maximumSteamRefreshErrorRunes])
	}
	return message
}
