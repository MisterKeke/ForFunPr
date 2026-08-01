package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

var ErrBackendNotReady = errors.New("backend not ready")

type Service struct {
	lifecycleMu      sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	db               *sql.DB
	ready            bool
	closing          bool
	active           sync.WaitGroup
	httpClient       *externalHTTPClient
	startupErr       error
	favoriteUpdateMu sync.RWMutex
	telegramPosts    *boundedTTLCache[[]TelegramPost]
	youTubeVideos    *boundedTTLCache[[]YouTubeVideo]
	youTubeHandles   *boundedTTLCache[string]
}

func NewService() *Service {
	return &Service{
		httpClient:     newExternalHTTPClient(),
		telegramPosts:  newBoundedTTLCache(telegramCacheCapacity, favoriteCacheTTL, cloneTelegramPosts),
		youTubeVideos:  newBoundedTTLCache(youTubeCacheCapacity, favoriteCacheTTL, cloneYouTubeVideos),
		youTubeHandles: newBoundedTTLCache[string](handleCacheCapacity, favoriteCacheTTL, nil),
	}
}

// Startup initialises persistent storage before the frontend uses the bound
// application methods. Schema management belongs to migrations.go; this
// lifecycle method deliberately contains no DDL.
func (a *Service) Startup(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	lifecycleContext, cancel := context.WithCancel(ctx)

	a.lifecycleMu.Lock()
	a.ctx = lifecycleContext
	a.cancel = cancel
	a.db = nil
	a.ready = false
	a.closing = false
	a.startupErr = nil
	a.lifecycleMu.Unlock()

	db, _, err := openDatabase(lifecycleContext)
	if err != nil {
		a.SetStartupError(fmt.Errorf("initialise local storage: %w", err))
		return
	}
	if _, err := applicationWallpaperDirectory(); err != nil {
		_ = db.Close()
		a.SetStartupError(fmt.Errorf("initialise wallpaper storage: %w", err))
		return
	}
	if _, err := applicationDesktopAppIconDirectory(); err != nil {
		_ = db.Close()
		a.SetStartupError(fmt.Errorf("initialise application icon storage: %w", err))
		return
	}

	a.lifecycleMu.Lock()
	a.db = db
	a.lifecycleMu.Unlock()

	if err := a.RecordAppOpen(); err != nil {
		a.SetStartupError(fmt.Errorf("record application open: %w", err))
		_ = a.Close()
		return
	}

	a.lifecycleMu.Lock()
	a.ready = true
	a.lifecycleMu.Unlock()
}

// StartupStatus gives the frontend a safe way to determine whether local
// storage is ready without exposing internal filesystem or SQLite errors.
type StartupStatus struct {
	Ready bool   `json:"ready"`
	Error string `json:"error,omitempty"`
}

func (a *Service) GetStartupStatus() StartupStatus {
	if a == nil {
		return StartupStatus{Error: "The application backend is unavailable."}
	}
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if a.startupErr != nil {
		return StartupStatus{
			Error: "The application services could not be started. Check local logs for details.",
		}
	}

	return StartupStatus{Ready: a.ready && !a.closing && a.db != nil}
}

// SetStartupError records a local-only startup failure, cancels active
// provider work, and prevents new UI/API operations from starting.
func (a *Service) SetStartupError(err error) {
	if a == nil || err == nil {
		return
	}
	a.lifecycleMu.Lock()
	a.startupErr = err
	a.ready = false
	if a.cancel != nil {
		a.cancel()
	}
	a.lifecycleMu.Unlock()
}

// BeginOperation pins the service lifecycle for one UI or REST operation.
// Shutdown first rejects new work, then waits for every returned done function.
func (a *Service) BeginOperation(ctx context.Context) (context.Context, func(), error) {
	if a == nil {
		return nil, nil, ErrBackendNotReady
	}
	if ctx == nil {
		ctx = context.Background()
	}

	a.lifecycleMu.Lock()
	if !a.ready || a.closing || a.db == nil || a.startupErr != nil {
		a.lifecycleMu.Unlock()
		return nil, nil, ErrBackendNotReady
	}
	lifecycleContext := a.ctx
	a.active.Add(1)
	a.lifecycleMu.Unlock()

	operationContext, cancel := context.WithCancel(ctx)
	stopLifecycleCancellation := context.AfterFunc(lifecycleContext, cancel)
	var once sync.Once
	done := func() {
		once.Do(func() {
			stopLifecycleCancellation()
			cancel()
			a.active.Done()
		})
	}
	return operationContext, done, nil
}

func (a *Service) Shutdown(ctx context.Context) {
	if a == nil {
		return
	}
	a.lifecycleMu.Lock()
	if a.closing {
		a.lifecycleMu.Unlock()
		return
	}
	a.closing = true
	a.ready = false
	if a.cancel != nil {
		a.cancel()
	}
	a.lifecycleMu.Unlock()

	a.active.Wait()
	if err := a.Close(); err != nil {
		println("Error closing database:", err.Error())
	}
}
