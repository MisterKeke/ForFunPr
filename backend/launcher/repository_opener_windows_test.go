//go:build windows

package launcher

import (
	"context"
	"testing"
)

func TestRepositoryEditorUsesOnlyTheResolvedRepositoryArgument(t *testing.T) {
	previous := startRepositoryEditor
	t.Cleanup(func() { startRepositoryEditor = previous })
	var executablePath string
	var repositoryPath string
	var launchContext context.Context
	startRepositoryEditor = func(ctx context.Context, executable string, repository string) error {
		launchContext = ctx
		executablePath = executable
		repositoryPath = repository
		return nil
	}

	if err := (windowsRepositoryOpener{}).OpenEditor(context.Background(), `C:\Editors\Editor.exe`, `C:\Repositories\Something`); err != nil {
		t.Fatal(err)
	}
	if executablePath != `C:\Editors\Editor.exe` || repositoryPath != `C:\Repositories\Something` {
		t.Fatalf("editor launch = executable %q repository %q", executablePath, repositoryPath)
	}
	if launchContext == nil {
		t.Fatal("editor launch did not receive a context")
	}
}

func TestRepositoryEditorRejectsCanceledContextBeforeLaunching(t *testing.T) {
	previous := startRepositoryEditor
	t.Cleanup(func() { startRepositoryEditor = previous })
	called := false
	startRepositoryEditor = func(context.Context, string, string) error {
		called = true
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (windowsRepositoryOpener{}).OpenEditor(ctx, `C:\Editors\Editor.exe`, `C:\Repositories\Something`); err == nil {
		t.Fatal("canceled editor launch unexpectedly succeeded")
	}
	if called {
		t.Fatal("canceled editor launch reached process execution")
	}
}
