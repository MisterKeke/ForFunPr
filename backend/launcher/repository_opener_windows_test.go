//go:build windows

package launcher

import "testing"

func TestRepositoryEditorUsesOnlyTheResolvedRepositoryArgument(t *testing.T) {
	previous := startRepositoryEditor
	t.Cleanup(func() { startRepositoryEditor = previous })
	var executablePath string
	var repositoryPath string
	startRepositoryEditor = func(executable string, repository string) error {
		executablePath = executable
		repositoryPath = repository
		return nil
	}

	if err := (windowsRepositoryOpener{}).OpenEditor(`C:\Editors\Editor.exe`, `C:\Repositories\Something`); err != nil {
		t.Fatal(err)
	}
	if executablePath != `C:\Editors\Editor.exe` || repositoryPath != `C:\Repositories\Something` {
		t.Fatalf("editor launch = executable %q repository %q", executablePath, repositoryPath)
	}
}
