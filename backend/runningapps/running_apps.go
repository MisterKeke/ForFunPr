package runningapps

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
)

const (
	wsExToolWindow uintptr = 0x00000080
	wsExAppWindow  uintptr = 0x00040000
)

var (
	ErrUnsupported       = errors.New("Running taskbar applications are supported only on Windows.")
	ErrInvalidWindowID   = errors.New("Choose a valid running app.")
	ErrWindowUnavailable = errors.New("That window is no longer available.")
	ErrActivationRefused = errors.New("Windows refused to activate that window. Try selecting it from the taskbar.")
)

type App struct {
	WindowID    string `json:"window_id"`
	Title       string `json:"title"`
	ProcessName string `json:"process_name,omitempty"`
	IconDataURL string `json:"icon_data_url,omitempty"`
	Minimized   bool   `json:"minimized"`
	Active      bool   `json:"active"`
}

type Provider interface {
	Supported() bool
	List(context.Context) ([]App, error)
	Activate(context.Context, string) error
}

type windowSnapshot struct {
	Handle                uintptr
	Title                 string
	ProcessName           string
	ProcessPath           string
	ProcessID             uint32
	ClassName             string
	ExStyle               uintptr
	Visible               bool
	Cloaked               bool
	Minimized             bool
	Active                bool
	TaskbarRepresentative bool
}

type iconCacheKey struct {
	WindowID  uintptr
	ProcessID uint32
}

func buildAppList(
	ctx context.Context,
	snapshots []windowSnapshot,
	currentProcessID uint32,
	resolveIcon func(context.Context, windowSnapshot) string,
) ([]App, map[iconCacheKey]struct{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	apps := make([]App, 0, len(snapshots))
	seenWindows := make(map[uintptr]struct{}, len(snapshots))
	seenIcons := make(map[iconCacheKey]struct{}, len(snapshots))
	for index, snapshot := range snapshots {
		if index%16 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
		}
		if !isTaskbarEligibleWindow(snapshot, currentProcessID) {
			continue
		}
		if _, exists := seenWindows[snapshot.Handle]; exists {
			continue
		}
		seenWindows[snapshot.Handle] = struct{}{}

		icon := ""
		if resolveIcon != nil {
			icon = resolveIcon(ctx, snapshot)
		}
		key := iconCacheKey{WindowID: snapshot.Handle, ProcessID: snapshot.ProcessID}
		seenIcons[key] = struct{}{}
		apps = append(apps, App{
			WindowID:    windowIDFromHandle(snapshot.Handle),
			Title:       strings.TrimSpace(snapshot.Title),
			ProcessName: strings.TrimSpace(snapshot.ProcessName),
			IconDataURL: icon,
			Minimized:   snapshot.Minimized,
			Active:      snapshot.Active,
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	sortApps(apps)
	return apps, seenIcons, nil
}

func isTaskbarEligibleWindow(snapshot windowSnapshot, currentProcessID uint32) bool {
	if snapshot.Handle == 0 {
		return false
	}
	if !snapshot.Visible || snapshot.Cloaked {
		return false
	}
	if currentProcessID != 0 && snapshot.ProcessID == currentProcessID {
		return false
	}
	if isShellSurfaceClass(snapshot.ClassName) {
		return false
	}

	hasAppWindow := snapshot.ExStyle&wsExAppWindow != 0
	hasToolWindow := snapshot.ExStyle&wsExToolWindow != 0
	if hasToolWindow && !hasAppWindow {
		return false
	}
	if !hasAppWindow && !snapshot.TaskbarRepresentative {
		return false
	}
	if strings.TrimSpace(snapshot.Title) == "" && strings.TrimSpace(snapshot.ProcessName) == "" {
		return false
	}
	return true
}

func isShellSurfaceClass(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "shell_traywnd", "shell_secondarytraywnd", "progman", "workerw":
		return true
	default:
		return false
	}
}

func sortApps(apps []App) {
	sort.SliceStable(apps, func(left, right int) bool {
		leftTitle := strings.ToLower(apps[left].Title)
		rightTitle := strings.ToLower(apps[right].Title)
		if leftTitle != rightTitle {
			return leftTitle < rightTitle
		}
		leftProcess := strings.ToLower(apps[left].ProcessName)
		rightProcess := strings.ToLower(apps[right].ProcessName)
		if leftProcess != rightProcess {
			return leftProcess < rightProcess
		}
		return apps[left].WindowID < apps[right].WindowID
	})
}

func resolveActivatableSnapshot(
	ctx context.Context,
	windowID string,
	snapshots []windowSnapshot,
	currentProcessID uint32,
) (windowSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	handle, err := parseWindowID(windowID)
	if err != nil {
		return windowSnapshot{}, err
	}
	for index, snapshot := range snapshots {
		if index%16 == 0 {
			if err := ctx.Err(); err != nil {
				return windowSnapshot{}, err
			}
		}
		if snapshot.Handle != handle {
			continue
		}
		if !isTaskbarEligibleWindow(snapshot, currentProcessID) {
			return windowSnapshot{}, ErrWindowUnavailable
		}
		return snapshot, nil
	}
	if err := ctx.Err(); err != nil {
		return windowSnapshot{}, err
	}
	return windowSnapshot{}, ErrWindowUnavailable
}

func parseWindowID(raw string) (uintptr, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return 0, ErrInvalidWindowID
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, ErrInvalidWindowID
		}
	}
	value, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || value == 0 {
		return 0, ErrInvalidWindowID
	}
	return uintptr(value), nil
}

func windowIDFromHandle(handle uintptr) string {
	return strconv.FormatUint(uint64(handle), 10)
}

func lastVisibleActivePopup(
	rootOwner uintptr,
	lastActivePopup func(uintptr) uintptr,
	isVisible func(uintptr) bool,
) uintptr {
	if rootOwner == 0 {
		return 0
	}
	current := rootOwner
	visited := make(map[uintptr]struct{}, 4)
	for current != 0 {
		if _, ok := visited[current]; ok {
			return current
		}
		visited[current] = struct{}{}
		popup := lastActivePopup(current)
		if popup == 0 || popup == current {
			return current
		}
		if isVisible(popup) {
			return popup
		}
		current = popup
	}
	return rootOwner
}
