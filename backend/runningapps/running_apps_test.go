package runningapps

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestTaskbarEligibilityFiltersWindowState(t *testing.T) {
	const currentProcessID = 10
	base := windowSnapshot{
		Handle:                100,
		Title:                 "Editor",
		ProcessName:           "editor.exe",
		ProcessID:             20,
		Visible:               true,
		TaskbarRepresentative: true,
	}

	tests := []struct {
		name     string
		mutate   func(*windowSnapshot)
		included bool
	}{
		{name: "visible normal window", included: true},
		{name: "invisible window", mutate: func(item *windowSnapshot) { item.Visible = false }},
		{name: "cloaked window", mutate: func(item *windowSnapshot) { item.Cloaked = true }},
		{name: "tool window", mutate: func(item *windowSnapshot) { item.ExStyle = wsExToolWindow }},
		{name: "tool app window", mutate: func(item *windowSnapshot) { item.ExStyle = wsExToolWindow | wsExAppWindow }, included: true},
		{name: "minimized window", mutate: func(item *windowSnapshot) { item.Minimized = true }, included: true},
		{name: "current process", mutate: func(item *windowSnapshot) { item.ProcessID = currentProcessID }},
		{name: "non representative owned window", mutate: func(item *windowSnapshot) { item.TaskbarRepresentative = false }},
		{name: "app window bypasses representative check", mutate: func(item *windowSnapshot) {
			item.TaskbarRepresentative = false
			item.ExStyle = wsExAppWindow
		}, included: true},
		{name: "shell surface", mutate: func(item *windowSnapshot) { item.ClassName = "Shell_TrayWnd" }},
		{name: "empty title and process", mutate: func(item *windowSnapshot) {
			item.Title = "  "
			item.ProcessName = ""
		}},
		{name: "process fallback", mutate: func(item *windowSnapshot) { item.Title = "" }, included: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := base
			if test.mutate != nil {
				test.mutate(&item)
			}
			if got := isTaskbarEligibleWindow(item, currentProcessID); got != test.included {
				t.Fatalf("included = %v, want %v", got, test.included)
			}
		})
	}
}

func TestBuildAppListDeduplicatesSortsAndKeepsIconFailures(t *testing.T) {
	snapshots := []windowSnapshot{
		{
			Handle: 3, Title: "Zulu", ProcessName: "z.exe", ProcessID: 30,
			Visible: true, TaskbarRepresentative: true,
		},
		{
			Handle: 1, Title: "alpha", ProcessName: "a.exe", ProcessID: 10,
			Visible: true, TaskbarRepresentative: true,
		},
		{
			Handle: 2, Title: "Alpha", ProcessName: "b.exe", ProcessID: 20,
			Visible: true, TaskbarRepresentative: true, Active: true,
		},
		{
			Handle: 1, Title: "alpha duplicate", ProcessName: "a.exe", ProcessID: 10,
			Visible: true, TaskbarRepresentative: true,
		},
	}

	apps, icons, err := buildAppList(context.Background(), snapshots, 99, func(context.Context, windowSnapshot) string {
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 3 {
		t.Fatalf("apps = %#v", apps)
	}
	if apps[0].WindowID != "1" || apps[1].WindowID != "2" || apps[2].WindowID != "3" {
		t.Fatalf("sort order = %#v", apps)
	}
	if apps[1].IconDataURL != "" || !apps[1].Active {
		t.Fatalf("icon failure or active state changed app = %#v", apps[1])
	}
	if len(icons) != 3 {
		t.Fatalf("tracked icon keys = %#v", icons)
	}
}

func TestLastVisibleActivePopupSelectsOwnedRepresentative(t *testing.T) {
	visible := map[uintptr]bool{11: false, 12: true}
	lastActive := map[uintptr]uintptr{10: 11, 11: 12, 12: 12}
	got := lastVisibleActivePopup(10, func(handle uintptr) uintptr {
		return lastActive[handle]
	}, func(handle uintptr) bool {
		return visible[handle]
	})
	if got != 12 {
		t.Fatalf("representative = %d, want 12", got)
	}

	visible[11] = true
	got = lastVisibleActivePopup(10, func(handle uintptr) uintptr {
		return lastActive[handle]
	}, func(handle uintptr) bool {
		return visible[handle]
	})
	if got != 11 {
		t.Fatalf("representative = %d, want 11", got)
	}

	apps, _, err := buildAppList(context.Background(), []windowSnapshot{
		{
			Handle: 10, Title: "Owner", ProcessName: "owner.exe", ProcessID: 20,
			Visible: true, TaskbarRepresentative: false,
		},
		{
			Handle: 11, Title: "Popup", ProcessName: "owner.exe", ProcessID: 20,
			Visible: true, TaskbarRepresentative: true,
		},
	}, 99, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].WindowID != "11" {
		t.Fatalf("owned popup app list = %#v, want only popup", apps)
	}
}

func TestResolveActivatableSnapshotRejectsBadStaleAndIneligibleIDs(t *testing.T) {
	snapshots := []windowSnapshot{
		{
			Handle: 7, Title: "Editor", ProcessName: "editor.exe", ProcessID: 20,
			Visible: true, TaskbarRepresentative: true,
		},
		{
			Handle: 8, Title: "Hidden", ProcessName: "hidden.exe", ProcessID: 30,
			Visible: false, TaskbarRepresentative: true,
		},
	}

	if got, err := resolveActivatableSnapshot(context.Background(), "7", snapshots, 10); err != nil || got.Handle != 7 {
		t.Fatalf("valid activation snapshot = %#v, %v", got, err)
	}
	for _, windowID := range []string{"", "0", "+7", " 7", "7 ", "abc", strconv.FormatUint(^uint64(0), 10) + "0"} {
		if _, err := resolveActivatableSnapshot(context.Background(), windowID, snapshots, 10); !errors.Is(err, ErrInvalidWindowID) {
			t.Fatalf("window ID %q error = %v, want ErrInvalidWindowID", windowID, err)
		}
	}
	for _, windowID := range []string{"8", "9"} {
		if _, err := resolveActivatableSnapshot(context.Background(), windowID, snapshots, 10); !errors.Is(err, ErrWindowUnavailable) {
			t.Fatalf("window ID %q error = %v, want ErrWindowUnavailable", windowID, err)
		}
	}
}
