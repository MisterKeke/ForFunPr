package backend

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	backendservice "something/backend/service"
)

type recordingGitRepositoryOpener struct {
	supported          bool
	folders            []string
	editorExecutables  []string
	editorRepositories []string
}

func (o *recordingGitRepositoryOpener) Supported() bool { return o.supported }

func (o *recordingGitRepositoryOpener) OpenFolder(path string) error {
	o.folders = append(o.folders, path)
	return nil
}

func (o *recordingGitRepositoryOpener) OpenEditor(executablePath string, repositoryPath string) error {
	o.editorExecutables = append(o.editorExecutables, executablePath)
	o.editorRepositories = append(o.editorRepositories, repositoryPath)
	return nil
}

type recordingGitRepositoryURLLauncher struct {
	values []string
}

func (l *recordingGitRepositoryURLLauncher) OpenURL(value string) error {
	l.values = append(l.values, value)
	return nil
}

func newGitWorkspaceBridgeTestApp(t *testing.T) *App {
	t.Helper()
	dataRoot := t.TempDir()
	t.Setenv("APPDATA", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", dataRoot)
	service := backendservice.NewServiceWithGitWorkspaceProvider(nil)
	service.Startup(context.Background())
	if status := service.GetStartupStatus(); !status.Ready {
		t.Fatalf("service startup = %#v", status)
	}
	t.Cleanup(func() { service.Shutdown(context.Background()) })
	return NewApp(service, nil)
}

func TestChooseGitWorkspaceFolderReturnsNilOnPickerCancellation(t *testing.T) {
	app := newGitWorkspaceBridgeTestApp(t)
	app.gitWorkspacePicker = func(context.Context) (string, error) {
		return "", nil
	}

	root, err := app.ChooseGitWorkspaceFolder()
	if err != nil {
		t.Fatal(err)
	}
	if root != nil {
		t.Fatalf("cancelled picker returned %#v", root)
	}
	roots, err := app.ListGitWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Fatalf("cancelled picker created roots: %#v", roots)
	}
}

func TestGitRepositoryOpenersResolvePathsFromIDsAndRejectMissingRepositories(t *testing.T) {
	app := newGitWorkspaceBridgeTestApp(t)
	opener := &recordingGitRepositoryOpener{supported: true}
	app.gitRepositoryOpener = opener
	repositoryPath := t.TempDir()
	repositories, err := app.service.UpsertGitRepositoriesContext(context.Background(), []backendservice.GitRepositoryUpsert{{
		Name: "Something", RepositoryPath: repositoryPath,
	}})
	if err != nil {
		t.Fatal(err)
	}
	repositoryID := repositories[0].ID

	if err := app.OpenGitRepositoryFolder(repositoryID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opener.folders, []string{filepath.Clean(repositoryPath)}) {
		t.Fatalf("opened folders = %#v", opener.folders)
	}

	if err := app.service.MarkGitRepositoriesMissingContext(context.Background(), []int{repositoryID}); err != nil {
		t.Fatal(err)
	}
	err = app.OpenGitRepositoryFolder(repositoryID)
	if err == nil || !strings.Contains(err.Error(), "no longer available") {
		t.Fatalf("missing repository error = %v", err)
	}
	if len(opener.folders) != 1 {
		t.Fatalf("missing repository reached native opener: %#v", opener.folders)
	}
}

func TestOpenGitRepositoryInEditorResolvesSavedEditorByID(t *testing.T) {
	app := newGitWorkspaceBridgeTestApp(t)
	opener := &recordingGitRepositoryOpener{supported: true}
	app.gitRepositoryOpener = opener
	repositoryPath := t.TempDir()
	editorPath := filepath.Join(t.TempDir(), "Editor.exe")
	if err := os.WriteFile(editorPath, []byte("editor"), 0o600); err != nil {
		t.Fatal(err)
	}
	repositories, err := app.service.UpsertGitRepositoriesContext(context.Background(), []backendservice.GitRepositoryUpsert{{
		Name: "Something", RepositoryPath: repositoryPath,
	}})
	if err != nil {
		t.Fatal(err)
	}
	editor, err := app.service.AddDesktopAppFromPathContext(context.Background(), editorPath)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := app.service.GetGitWorkspaceSettingsContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.service.UpdateGitWorkspaceSettingsContext(context.Background(), backendservice.GitWorkspaceSettingsUpdateRequest{
		EditorApplicationID: &editor.ID,
		WorkerCount:         settings.WorkerCount,
		StaleDays:           settings.StaleDays,
		ExpectedRevision:    settings.Revision,
	}); err != nil {
		t.Fatal(err)
	}

	if err := app.OpenGitRepositoryInEditor(repositories[0].ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opener.editorExecutables, []string{editorPath}) || !reflect.DeepEqual(opener.editorRepositories, []string{filepath.Clean(repositoryPath)}) {
		t.Fatalf("editor open calls = executables %#v repositories %#v", opener.editorExecutables, opener.editorRepositories)
	}
}

func TestOpenGitRepositoryRemoteUsesValidatedRemoteWebURL(t *testing.T) {
	app := newGitWorkspaceBridgeTestApp(t)
	launcher := &recordingGitRepositoryURLLauncher{}
	app.externalURLLauncher = launcher
	repositories, err := app.service.UpsertGitRepositoriesContext(context.Background(), []backendservice.GitRepositoryUpsert{{
		Name: "Something", RepositoryPath: t.TempDir(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.service.WriteGitRepositoryStatusContext(context.Background(), backendservice.GitRepositoryStatusWriteRequest{
		RepositoryID: repositories[0].ID,
		RemoteWebURL: "https://example.com/something",
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenGitRepositoryRemote(repositories[0].ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(launcher.values, []string{"https://example.com/something"}) {
		t.Fatalf("opened URLs = %#v", launcher.values)
	}

	app.externalURLLauncher = nil
	err = app.OpenGitRepositoryRemote(repositories[0].ID)
	if err == nil || !strings.Contains(err.Error(), "opening web pages is unavailable") {
		t.Fatalf("invalid remote opener error = %v", err)
	}

	app.externalURLLauncher = &recordingGitRepositoryURLLauncher{}
	if err := app.service.WriteGitRepositoryStatusContext(context.Background(), backendservice.GitRepositoryStatusWriteRequest{
		RepositoryID: repositories[0].ID,
		RemoteWebURL: "",
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenGitRepositoryRemote(repositories[0].ID); err == nil {
		t.Fatal("empty sanitized remote URL was opened")
	}
}

func TestGitRepositoryOpenersRejectUnavailableNativeBoundary(t *testing.T) {
	app := newGitWorkspaceBridgeTestApp(t)
	app.gitRepositoryOpener = &recordingGitRepositoryOpener{}
	repositories, err := app.service.UpsertGitRepositoriesContext(context.Background(), []backendservice.GitRepositoryUpsert{{
		Name: "Something", RepositoryPath: t.TempDir(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.OpenGitRepositoryFolder(repositories[0].ID)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unsupported opener error = %v", err)
	}
}
