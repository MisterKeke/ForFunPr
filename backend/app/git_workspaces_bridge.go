package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"something/backend/actions"
	backendservice "something/backend/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ListGitWorkspaces() ([]GitWorkspaceRoot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListGitWorkspaceRootsContext(ctx)
}

// ChooseGitWorkspaceFolder uses the native picker so JavaScript cannot supply
// an arbitrary path. A nil result means the picker was cancelled.
func (a *App) ChooseGitWorkspaceFolder() (*GitWorkspaceRoot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()

	picker := a.gitWorkspacePicker
	if picker == nil {
		return nil, errors.New("the Git workspace folder picker is unavailable")
	}
	selectedPath, err := picker(ctx)
	if err != nil {
		return nil, errors.New("the Git workspace folder picker could not be opened")
	}
	if strings.TrimSpace(selectedPath) == "" {
		return nil, nil
	}

	displayName := filepath.Base(filepath.Clean(selectedPath))
	root, err := service.AddGitWorkspaceRootContext(ctx, backendservice.GitWorkspaceRootCreateRequest{
		DisplayName: displayName,
		RootPath:    selectedPath,
	})
	if err != nil {
		return nil, err
	}
	return &root, nil
}

func chooseGitWorkspaceFolder(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Choose a Git workspace folder",
	})
}

func (a *App) RemoveGitWorkspace(request GitWorkspaceRootDeleteRequest) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.RemoveGitWorkspaceRootContext(ctx, request)
}

func (a *App) StartGitWorkspaceRescan(request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.StartGitWorkspaceRescanJobContext(ctx, request)
}

func (a *App) ListGitRepositories(filter GitRepositoryListFilter) ([]GitRepository, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListGitRepositoriesContext(ctx, filter)
}

func (a *App) StartGitStatusRefresh(request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.StartGitWorkspaceStatusRefreshJobContext(ctx, request)
}

func (a *App) GetGitRepositoryDetails(repositoryID int) (GitWorkspaceRepositoryDetails, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceRepositoryDetails{}, err
	}
	defer done()
	return service.GetGitRepositoryDetailsContext(ctx, repositoryID)
}

func (a *App) GetGitRepositoryHistory(repositoryID int, limit int) ([]GitWorkspaceCommit, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetGitRepositoryHistoryContext(ctx, repositoryID, limit)
}

func (a *App) StartGitFetch(request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.StartGitWorkspaceFetchJobContext(ctx, request)
}

func (a *App) StartGitPull(request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.StartGitWorkspacePullJobContext(ctx, request)
}

func (a *App) StartGitSync(request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.StartGitWorkspaceSyncJobContext(ctx, request)
}

func (a *App) GetGitWorkspaceJob(jobID string) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.GetGitWorkspaceJobContext(ctx, jobID)
}

func (a *App) CancelGitWorkspaceJob(jobID string) (GitWorkspaceJob, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	defer done()
	return service.CancelGitWorkspaceJobContext(ctx, jobID)
}

func (a *App) PruneMissingGitRepositories() (int, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return 0, err
	}
	defer done()
	return service.PruneMissingGitWorkspaceRepositoriesContext(ctx)
}

func (a *App) GetGitWorkspaceSettings() (GitWorkspaceSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceSettings{}, err
	}
	defer done()
	return service.GetGitWorkspaceSettingsContext(ctx)
}

func (a *App) UpdateGitWorkspaceSettings(request GitWorkspaceSettingsUpdateRequest) (GitWorkspaceSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return GitWorkspaceSettings{}, err
	}
	defer done()
	return service.UpdateGitWorkspaceSettingsContext(ctx, request)
}

func (a *App) OpenGitRepositoryFolder(repositoryID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	path, err := resolveGitRepositoryFolder(ctx, service, repositoryID)
	if err != nil {
		return err
	}
	if a.gitRepositoryOpener == nil || !a.gitRepositoryOpener.Supported() {
		return errors.New("opening repository folders is unavailable on this system")
	}
	if err := a.gitRepositoryOpener.OpenFolder(path); err != nil {
		return errors.New("the repository folder could not be opened")
	}
	return nil
}

func (a *App) OpenGitRepositoryInEditor(repositoryID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	path, err := resolveGitRepositoryFolder(ctx, service, repositoryID)
	if err != nil {
		return err
	}
	settings, err := service.GetGitWorkspaceSettingsContext(ctx)
	if err != nil {
		return err
	}
	if settings.EditorApplicationID == nil {
		return &backendservice.ValidationError{Field: "editor_application_id", Message: "Choose a saved editor first."}
	}
	executablePath, err := service.DesktopAppExecutableContext(ctx, *settings.EditorApplicationID)
	if err != nil {
		return err
	}
	if a.gitRepositoryOpener == nil || !a.gitRepositoryOpener.Supported() {
		return errors.New("opening repositories in an editor is unavailable on this system")
	}
	if err := a.gitRepositoryOpener.OpenEditor(executablePath, path); err != nil {
		return errors.New("the repository could not be opened in the configured editor")
	}
	return nil
}

func (a *App) OpenGitRepositoryRemote(repositoryID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	status, err := service.ReadGitRepositoryStatusContext(ctx, repositoryID)
	if err != nil {
		return err
	}
	return actions.OpenExternalURL(ctx, a.externalURLLauncher, status.RemoteWebURL)
}

func resolveGitRepositoryFolder(ctx context.Context, service *backendservice.Service, repositoryID int) (string, error) {
	repository, err := service.GetGitRepositoryContext(ctx, repositoryID)
	if err != nil {
		return "", err
	}
	if repository.Missing {
		return "", &backendservice.ValidationError{Field: "repository_id", Message: "That repository is no longer available."}
	}
	path := filepath.Clean(strings.TrimSpace(repository.RepositoryPath))
	if path == "." || !filepath.IsAbs(path) {
		return "", &backendservice.ValidationError{Field: "repository_id", Message: "That repository is no longer available."}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", &backendservice.ValidationError{Field: "repository_id", Message: "That repository is no longer available."}
	}
	if !info.IsDir() {
		return "", &backendservice.ValidationError{Field: "repository_id", Message: "That repository is no longer available."}
	}
	return path, nil
}
