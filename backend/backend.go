package backend

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type App struct {
	ctx                      context.Context
	db                       *sql.DB
	httpClient               *externalHTTPClient
	startupErr               error
	favoriteUpdateMu         sync.Mutex
	lastFavoriteUpdateMu     sync.RWMutex
	lastFavoriteUpdateResult FavoriteUpdateScanResult
}

func NewApp() *App {
	return &App{
		httpClient: newExternalHTTPClient(),
	}
}

// Startup initialises persistent storage before the frontend uses the bound
// application methods. Schema management belongs to migrations.go; this
// lifecycle method deliberately contains no DDL.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	db, _, err := openDatabase(ctx)
	if err != nil {
		a.startupErr = fmt.Errorf("initialise local storage: %w", err)
		return
	}

	a.db = db

	if err := a.RecordAppOpen(); err != nil {
		a.startupErr = fmt.Errorf("record application open: %w", err)
		_ = a.Close()
	}
}

// StartupStatus gives the frontend a safe way to determine whether local
// storage is ready without exposing internal filesystem or SQLite errors.
type StartupStatus struct {
	Ready bool   `json:"ready"`
	Error string `json:"error,omitempty"`
}

func (a *App) GetStartupStatus() StartupStatus {
	if a.startupErr != nil {
		return StartupStatus{
			Error: "Local storage could not be initialized. Check that the application data directory is writable.",
		}
	}

	return StartupStatus{Ready: a.db != nil}
}

func (a *App) Shutdown(ctx context.Context) {
	if err := a.Close(); err != nil {
		println("Error closing database:", err.Error())
	}
}
