package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	backendservice "something/backend/service"
)

type recordingURLLauncher struct {
	values []string
	err    error
}

func (launcher *recordingURLLauncher) OpenURL(value string) error {
	launcher.values = append(launcher.values, value)
	return launcher.err
}

type recordingAppLauncher struct {
	paths  []string
	failAt map[string]error
}

func (launcher *recordingAppLauncher) Launch(path string) error {
	launcher.paths = append(launcher.paths, path)
	return launcher.failAt[path]
}

func newActionTestService(t *testing.T) *backendservice.Service {
	t.Helper()
	dataRoot := t.TempDir()
	t.Setenv("APPDATA", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", dataRoot)
	service := backendservice.NewService()
	service.Startup(context.Background())
	if status := service.GetStartupStatus(); !status.Ready {
		t.Fatalf("service startup = %#v", status)
	}
	t.Cleanup(func() { service.Shutdown(context.Background()) })
	return service
}

func writeActionTestExecutable(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".exe")
	if err := os.WriteFile(path, []byte("test executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenExternalURLValidatesBeforeDelegating(t *testing.T) {
	launcher := &recordingURLLauncher{}
	if err := OpenExternalURL(context.Background(), launcher, "  HTTP://Example.com/a?q=1  "); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(launcher.values, []string{"http://Example.com/a?q=1"}) {
		t.Fatalf("opened URLs = %#v", launcher.values)
	}

	invalid := []string{
		"",
		"ftp://example.com/file",
		"https://user:secret@example.com/",
		"https://example.com/\nnext",
		strings.Repeat("x", maximumExternalURLBytes+1),
	}
	for _, value := range invalid {
		before := len(launcher.values)
		err := OpenExternalURL(context.Background(), launcher, value)
		var validation *backendservice.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("OpenExternalURL(%q) error = %v, want ValidationError", value, err)
		}
		if len(launcher.values) != before {
			t.Fatalf("invalid URL %q reached launcher", value)
		}
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := OpenExternalURL(canceled, launcher, "https://example.com"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled OpenExternalURL = %v", err)
	}
}

func TestOpenExternalURLHidesLauncherFailure(t *testing.T) {
	launcher := &recordingURLLauncher{err: errors.New("sensitive platform error")}
	err := OpenExternalURL(context.Background(), launcher, "https://example.com/path")
	if err == nil || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "example.com") {
		t.Fatalf("launcher error was not safely mapped: %v", err)
	}
}

func TestLaunchDesktopAppUsesOnlyTheSavedExecutable(t *testing.T) {
	service := newActionTestService(t)
	path := writeActionTestExecutable(t, "Saved App")
	app, err := service.AddDesktopAppFromPathContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	launcher := &recordingAppLauncher{failAt: map[string]error{}}

	result, err := LaunchDesktopApp(context.Background(), service, launcher, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppID != app.ID || result.AppName != app.DisplayName || !result.Launched {
		t.Fatalf("launch result = %#v", result)
	}
	if !reflect.DeepEqual(launcher.paths, []string{path}) {
		t.Fatalf("launcher paths = %#v", launcher.paths)
	}

	launcher.failAt[path] = errors.New("platform details")
	_, err = LaunchDesktopApp(context.Background(), service, launcher, app.ID)
	if err == nil || strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "platform details") {
		t.Fatalf("unsafe launch error = %v", err)
	}
	if _, err := LaunchDesktopApp(context.Background(), service, launcher, 999999); err == nil {
		t.Fatal("missing desktop application was accepted")
	}
}

func TestStartSetupPreservesOrderAndContinuesAfterLaunchFailure(t *testing.T) {
	service := newActionTestService(t)
	paths := []string{
		writeActionTestExecutable(t, "First"),
		writeActionTestExecutable(t, "Second"),
		writeActionTestExecutable(t, "Third"),
	}
	appIDs := make([]int, 0, len(paths))
	for _, path := range paths {
		app, err := service.AddDesktopAppFromPathContext(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		appIDs = append(appIDs, app.ID)
	}
	setup, err := service.CreateSetupContext(context.Background(), backendservice.SetupCreateRequest{
		Name: "Daily", AppIDs: appIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	launcher := &recordingAppLauncher{failAt: map[string]error{
		paths[1]: errors.New("second launch failed"),
	}}

	result, err := StartSetup(context.Background(), service, launcher, setup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Attempted != 3 || result.Launched != 2 || len(result.Failures) != 1 {
		t.Fatalf("setup result = %#v", result)
	}
	if result.Failures[0].AppID != appIDs[1] || result.Failures[0].Error != "Windows could not open this application" {
		t.Fatalf("setup failure = %#v", result.Failures[0])
	}
	if !reflect.DeepEqual(launcher.paths, paths) {
		t.Fatalf("launch order = %#v, want %#v", launcher.paths, paths)
	}
}

func TestNativeActionsRejectUnavailableDependencies(t *testing.T) {
	if _, err := LaunchDesktopApp(nil, nil, nil, 1); err == nil {
		t.Fatal("LaunchDesktopApp accepted unavailable dependencies")
	}
	if _, err := StartSetup(nil, nil, nil, 1); err == nil {
		t.Fatal("StartSetup accepted unavailable dependencies")
	}
	if err := OpenExternalURL(nil, nil, "https://example.com"); err == nil {
		t.Fatal("OpenExternalURL accepted a nil launcher")
	}
}
