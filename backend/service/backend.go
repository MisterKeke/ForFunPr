package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"something/backend/storage"
)

var ErrBackendNotReady = errors.New("backend not ready")

type Service struct {
	lifecycleMu               sync.Mutex
	ctx                       context.Context
	cancel                    context.CancelFunc
	db                        *sql.DB
	ready                     bool
	closing                   bool
	active                    sync.WaitGroup
	httpClient                *externalHTTPClient
	websiteHTTPClient         *websiteHTTPClient
	startupErr                error
	capabilities              map[string]CapabilityState
	clock                     func() time.Time
	localTimeZone             *time.Location
	favoriteUpdateMu          sync.RWMutex
	steamGameRefreshMu        sync.Mutex
	steamInitialRefreshDone   bool
	steamInitialRefreshResult SteamGameRefreshResult
	steamInitialRefreshErr    error
	telegramPosts             *boundedTTLCache[[]TelegramPost]
	youTubeVideos             *boundedTTLCache[[]YouTubeVideo]
	youTubeHandles            *boundedTTLCache[string]
}

func NewService() *Service {
	return &Service{
		httpClient:        newExternalHTTPClient(),
		websiteHTTPClient: newWebsiteHTTPClient(),
		telegramPosts:     newBoundedTTLCache(telegramCacheCapacity, favoriteCacheTTL, cloneTelegramPosts),
		youTubeVideos:     newBoundedTTLCache(youTubeCacheCapacity, favoriteCacheTTL, cloneYouTubeVideos),
		youTubeHandles:    newBoundedTTLCache[string](handleCacheCapacity, favoriteCacheTTL, nil),
		clock:             time.Now,
		localTimeZone:     time.Local,
		capabilities:      make(map[string]CapabilityState),
	}
}

func (a *Service) now() time.Time {
	if a != nil && a.clock != nil {
		location := a.localTimeZone
		if location == nil {
			location = time.Local
		}
		return a.clock().In(location)
	}
	return time.Now().In(time.Local)
}

// SetClock replaces the internal date source. It is intended for deterministic
// service tests and embeds; production callers normally leave the default.
func (a *Service) SetClock(clock func() time.Time, location *time.Location) {
	if a == nil {
		return
	}
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if clock != nil {
		a.clock = clock
	}
	if location != nil {
		a.localTimeZone = location
	}
}

// Startup initialises persistent storage before the frontend uses the bound
// application methods. Schema management belongs to the storage package; this
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
	a.capabilities = defaultCapabilityStates()
	a.lifecycleMu.Unlock()

	a.steamGameRefreshMu.Lock()
	a.steamInitialRefreshDone = false
	a.steamInitialRefreshResult = SteamGameRefreshResult{}
	a.steamInitialRefreshErr = nil
	a.steamGameRefreshMu.Unlock()

	db, _, err := storage.Open(lifecycleContext)
	if err != nil {
		a.SetStartupError(fmt.Errorf("initialise local storage: %w", err))
		return
	}
	a.lifecycleMu.Lock()
	a.db = db
	a.lifecycleMu.Unlock()

	optionalDirectories := []struct {
		name    string
		warning string
		open    func() (string, error)
	}{
		{"wallpaper", "Wallpaper storage is unavailable.", storage.WallpaperDirectory},
		{"desktop_app_icons", "Desktop application icons are unavailable.", storage.DesktopAppIconDirectory},
		{"setup_icons", "Setup icons are unavailable.", storage.SetupIconDirectory},
		{"steam_artwork", "Steam artwork is unavailable.", storage.SteamGameImageDirectory},
		{"screenshots", "Screenshot storage is unavailable.", storage.ScreenshotDirectory},
	}
	for _, capability := range optionalDirectories {
		if _, err := capability.open(); err != nil {
			slog.Warn("Optional storage capability could not be initialized", "capability", capability.name, "error", err)
			a.SetCapability(capability.name, false, false, true, capability.warning)
			continue
		}
		a.SetCapability(capability.name, true, false, false, "")
	}
	if a.CapabilityAvailable("screenshots") {
		if err := a.CleanupOrphanedScreenshotFilesContext(lifecycleContext); err != nil {
			slog.Warn("Screenshot orphan cleanup failed", "error", err)
		}
		if err := a.RecoverInterruptedScreenshotOCRContext(lifecycleContext); err != nil {
			slog.Warn("Screenshot OCR recovery failed", "error", err)
			a.SetCapability("ocr", false, false, true, "Screenshot text recognition is temporarily unavailable.")
		} else {
			a.SetCapability("ocr", true, false, false, "")
		}
	} else {
		a.SetCapability("ocr", false, false, true, "Screenshot text recognition requires screenshot storage.")
	}

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
	Ready        bool                       `json:"ready"`
	CoreReady    bool                       `json:"core_ready"`
	Error        string                     `json:"error,omitempty"`
	Capabilities map[string]CapabilityState `json:"capabilities"`
	Warnings     []string                   `json:"warnings"`
}

type CapabilityState struct {
	Available bool   `json:"available"`
	Running   bool   `json:"running"`
	Retryable bool   `json:"retryable"`
	Warning   string `json:"warning,omitempty"`
}

func (a *Service) GetStartupStatus() StartupStatus {
	if a == nil {
		return StartupStatus{Error: "The application backend is unavailable.", Capabilities: map[string]CapabilityState{}}
	}
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if a.startupErr != nil {
		return StartupStatus{Error: "The application services could not be started. Check local logs for details.", Capabilities: cloneCapabilityStates(a.capabilities)}
	}
	ready := a.ready && !a.closing && a.db != nil
	capabilities := cloneCapabilityStates(a.capabilities)
	warnings := make([]string, 0)
	for _, name := range capabilityNames() {
		if warning := capabilities[name].Warning; warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return StartupStatus{Ready: ready, CoreReady: ready, Capabilities: capabilities, Warnings: warnings}
}

func (a *Service) SetCapability(name string, available, running, retryable bool, warning string) {
	if a == nil || name == "" {
		return
	}
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.capabilities == nil {
		a.capabilities = make(map[string]CapabilityState)
	}
	a.capabilities[name] = CapabilityState{Available: available, Running: running, Retryable: retryable, Warning: warning}
}

func (a *Service) CapabilityAvailable(name string) bool {
	if a == nil {
		return false
	}
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	return a.capabilities[name].Available
}

func capabilityNames() []string {
	return []string{"wallpaper", "desktop_app_icons", "setup_icons", "steam_artwork", "screenshots", "ocr", "clipboard", "file_shell", "launcher", "desktop_api"}
}

func defaultCapabilityStates() map[string]CapabilityState {
	states := make(map[string]CapabilityState)
	for _, name := range capabilityNames() {
		states[name] = CapabilityState{Retryable: true}
	}
	return states
}

func cloneCapabilityStates(source map[string]CapabilityState) map[string]CapabilityState {
	result := make(map[string]CapabilityState, len(source))
	for name, state := range source {
		result[name] = state
	}
	return result
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
	if ctx == nil {
		ctx = context.Background()
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

	operationsDone := make(chan struct{})
	go func() {
		a.active.Wait()
		close(operationsDone)
	}()

	select {
	case <-operationsDone:
	case <-ctx.Done():
		// Do not close SQLite underneath an active operation. Finish cleanup in
		// the background once every operation has released its lifecycle pin.
		go func() {
			<-operationsDone
			if err := a.Close(); err != nil {
				println("Error closing database:", err.Error())
			}
		}()
		return
	}
	if err := a.Close(); err != nil {
		println("Error closing database:", err.Error())
	}
}

// Close releases the SQLite connection during application shutdown.
func (a *Service) Close() error {
	if a == nil {
		return nil
	}
	if a.httpClient != nil {
		a.httpClient.closeIdleConnections()
	}
	if a.websiteHTTPClient != nil {
		a.websiteHTTPClient.closeIdleConnections()
	}

	a.lifecycleMu.Lock()
	if a.db == nil {
		a.lifecycleMu.Unlock()
		return nil
	}

	db := a.db
	a.db = nil
	a.ready = false
	a.lifecycleMu.Unlock()
	return db.Close()
}
