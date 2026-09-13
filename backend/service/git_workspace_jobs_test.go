package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"something/backend/gitworkspace"
	"something/backend/storage"
)

type fakeGitWorkspaceProvider struct {
	detect       gitworkspace.EnvironmentCapability
	status       func(context.Context, gitworkspace.Repository) (gitworkspace.Status, error)
	fetch        func(context.Context, []gitworkspace.Repository) (gitworkspace.BatchResult, error)
	pull         func(context.Context, []gitworkspace.Repository) (gitworkspace.BatchResult, error)
	sync         func(context.Context, []gitworkspace.Repository) (gitworkspace.SynchronizationResult, error)
	scan         func(context.Context, string) (gitworkspace.ScanResult, error)
	history      func(context.Context, gitworkspace.Repository, int) ([]gitworkspace.Commit, error)
	fetchSeen    chan struct{}
	release      chan struct{}
	ignoreCancel bool
}

func (f *fakeGitWorkspaceProvider) Detect(context.Context) (gitworkspace.EnvironmentCapability, error) {
	return f.detect, nil
}

func (f *fakeGitWorkspaceProvider) Scan(ctx context.Context, root string, _ ...gitworkspace.ScanOptions) (gitworkspace.ScanResult, error) {
	if f.scan != nil {
		return f.scan(ctx, root)
	}
	return gitworkspace.ScanResult{Repositories: []gitworkspace.Repository{}}, nil
}

func (f *fakeGitWorkspaceProvider) Status(ctx context.Context, repository gitworkspace.Repository) (gitworkspace.Status, error) {
	if f.status != nil {
		return f.status(ctx, repository)
	}
	return gitworkspace.Status{Repository: repository}, nil
}

func (f *fakeGitWorkspaceProvider) History(ctx context.Context, repository gitworkspace.Repository, limit int) ([]gitworkspace.Commit, error) {
	if f.history != nil {
		return f.history(ctx, repository, limit)
	}
	return []gitworkspace.Commit{}, nil
}

func (f *fakeGitWorkspaceProvider) Fetch(ctx context.Context, repositories []gitworkspace.Repository) (gitworkspace.BatchResult, error) {
	if f.fetchSeen != nil {
		select {
		case f.fetchSeen <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		if f.ignoreCancel {
			<-f.release
		} else {
			select {
			case <-f.release:
			case <-ctx.Done():
				return gitworkspace.BatchResult{}, ctx.Err()
			}
		}
	}
	if f.fetch != nil {
		return f.fetch(ctx, repositories)
	}
	return gitworkspace.BatchResult{Results: []gitworkspace.OperationResult{{Repository: repositories[0], Outcome: gitworkspace.OutcomeUpdated}}}, nil
}

func (f *fakeGitWorkspaceProvider) Pull(ctx context.Context, repositories []gitworkspace.Repository) (gitworkspace.BatchResult, error) {
	if f.pull != nil {
		return f.pull(ctx, repositories)
	}
	return gitworkspace.BatchResult{Results: []gitworkspace.OperationResult{{Repository: repositories[0], Outcome: gitworkspace.OutcomeAlreadyUpToDate}}}, nil
}

func (f *fakeGitWorkspaceProvider) Sync(ctx context.Context, repositories []gitworkspace.Repository) (gitworkspace.SynchronizationResult, error) {
	if f.sync != nil {
		return f.sync(ctx, repositories)
	}
	return gitworkspace.SynchronizationResult{}, nil
}

func newGitJobTestService(t *testing.T, provider GitWorkspaceProvider) *Service {
	t.Helper()
	db, err := storage.OpenInMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithGitWorkspaceProvider(provider)
	lifecycleContext, cancel := context.WithCancel(context.Background())
	service.db = db
	service.ctx = lifecycleContext
	service.cancel = cancel
	service.ready = true
	t.Cleanup(func() { service.Shutdown(context.Background()) })
	return service
}

func waitForGitWorkspaceJob(t *testing.T, service *Service, id string) GitWorkspaceJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.GetGitWorkspaceJobContext(context.Background(), id)
		if err == nil && (job.State == GitWorkspaceJobComplete || job.State == GitWorkspaceJobFailed || job.State == GitWorkspaceJobCancelled) {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job %s did not finish", id)
	return GitWorkspaceJob{}
}

func TestGitWorkspaceStatusJobCachesPartialResultsWithoutFetching(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.status = func(_ context.Context, repository gitworkspace.Repository) (gitworkspace.Status, error) {
		if repository.Name == "bad" {
			return gitworkspace.Status{}, errors.New("secret path and stderr")
		}
		return gitworkspace.Status{Repository: repository, Branch: gitworkspace.Branch{Name: "main"}, Sync: gitworkspace.SyncStatus{State: gitworkspace.SyncClean}}, nil
	}
	service := newGitJobTestService(t, provider)
	good, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{Name: "good", RepositoryPath: `C:\good`}})
	if err != nil {
		t.Fatal(err)
	}
	bad, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{Name: "bad", RepositoryPath: `C:\bad`}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartGitWorkspaceStatusRefreshContext(context.Background(), GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForGitWorkspaceJob(t, service, job.ID)
	if finished.Succeeded != 1 || finished.Failed != 1 || len(finished.Outcomes) != 2 {
		t.Fatalf("partial status job = %#v", finished)
	}
	for _, outcome := range finished.Outcomes {
		if strings.Contains(outcome.Error, "secret") {
			t.Fatalf("provider diagnostic leaked: %#v", finished.Outcomes)
		}
	}
	if _, err := service.ReadGitRepositoryStatusContext(context.Background(), good[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReadGitRepositoryStatusContext(context.Background(), bad[0].ID); err == nil {
		t.Fatal("failed status unexpectedly populated the cache")
	}
}

func TestGitWorkspaceStatusSanitizesProviderRemoteBeforeCaching(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.status = func(_ context.Context, repository gitworkspace.Repository) (gitworkspace.Status, error) {
		return gitworkspace.Status{
			Repository: repository,
			Remote:     "http://user:secret@example.com/org/repo?token=secret#fragment",
		}, nil
	}
	service := newGitJobTestService(t, provider)
	repositories, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{
		Name: "repo", RepositoryPath: `C:\repo`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartGitWorkspaceStatusRefreshContext(context.Background(), GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	waitForGitWorkspaceJob(t, service, job.ID)
	status, err := service.ReadGitRepositoryStatusContext(context.Background(), repositories[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.RemoteDisplay != "http://example.com/org/repo" || status.RemoteWebURL != "http://example.com/org/repo" {
		t.Fatalf("provider remote was not sanitized: %#v", status)
	}
}

func TestGitWorkspaceJobsRejectDuplicateMutationsAndShutdownWaits(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	provider := &fakeGitWorkspaceProvider{fetchSeen: started, release: release, ignoreCancel: true}
	service := newGitJobTestService(t, provider)
	if _, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{Name: "repo", RepositoryPath: `C:\repo`}}); err != nil {
		t.Fatal(err)
	}
	first, err := service.StartGitWorkspaceFetchContext(context.Background(), GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := service.StartGitWorkspacePullContext(context.Background(), GitRepositoryListFilter{}); err == nil {
		t.Fatal("conflicting mutation unexpectedly started")
	} else {
		var conflict *ConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("conflicting mutation error = %v", err)
		}
	}
	shutdownDone := make(chan struct{})
	go func() {
		service.Shutdown(context.Background())
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before provider work released")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wait for Git workspace job")
	}
	finished := waitForGitWorkspaceJob(t, service, first.ID)
	if finished.State != GitWorkspaceJobCancelled {
		t.Fatalf("shutdown job state = %s", finished.State)
	}
}

func TestGitWorkspaceJobCancellationPropagatesToProvider(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.fetch = func(ctx context.Context, repositories []gitworkspace.Repository) (gitworkspace.BatchResult, error) {
		<-ctx.Done()
		return gitworkspace.BatchResult{}, ctx.Err()
	}
	service := newGitJobTestService(t, provider)
	if _, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{Name: "repo", RepositoryPath: `C:\repo`}}); err != nil {
		t.Fatal(err)
	}
	job, err := service.StartGitWorkspaceFetchContext(context.Background(), GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CancelGitWorkspaceJobContext(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	finished := waitForGitWorkspaceJob(t, service, job.ID)
	if finished.State != GitWorkspaceJobCancelled {
		t.Fatalf("cancelled job state = %s", finished.State)
	}
}

func TestGitWorkspaceWorkersStopDispatchingQueuedRepositoriesAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var seenMu sync.Mutex
	seen := 0
	repositories := []GitRepository{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}
	go func() {
		runGitWorkspaceRepositoryWorkers(ctx, 1, repositories, func(GitRepository) {
			seenMu.Lock()
			seen++
			seenMu.Unlock()
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
		})
	}()
	<-started
	cancel()
	close(release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		seenMu.Lock()
		count := seen
		seenMu.Unlock()
		if count == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	seenMu.Lock()
	defer seenMu.Unlock()
	t.Fatalf("canceled worker dispatched %d repositories, want 1", seen)
}

func TestGitWorkspaceScanDoesNotPersistPartialProviderResults(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.scan = func(context.Context, string) (gitworkspace.ScanResult, error) {
		return gitworkspace.ScanResult{
			Repositories: []gitworkspace.Repository{{Name: "partial", Path: `C:\partial`}},
		}, errors.New("scan stopped after a provider failure")
	}
	service := newGitJobTestService(t, provider)
	root, err := service.CreateGitWorkspaceRootContext(context.Background(), GitWorkspaceRootCreateRequest{
		DisplayName: "Workspace", RootPath: `C:\workspace`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileGitWorkspaceScanContext(context.Background(), GitWorkspaceScanReconcileRequest{
		WorkspaceID:  root.ID,
		Repositories: []GitRepositoryScanResult{{Name: "existing", RepositoryPath: `C:\existing`}},
	}); err != nil {
		t.Fatal(err)
	}

	job, err := service.StartGitWorkspaceScanContext(context.Background(), root.ID)
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForGitWorkspaceJob(t, service, job.ID)
	if finished.State != GitWorkspaceJobFailed {
		t.Fatalf("partial scan state = %s, want failed", finished.State)
	}
	repositories, err := service.ListGitRepositoriesContext(context.Background(), GitRepositoryListFilter{WorkspaceID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].Name != "existing" {
		t.Fatalf("partial scan replaced inventory: %#v", repositories)
	}
}

func TestGitWorkspacePullPreservesSafetySkips(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.pull = func(_ context.Context, repositories []gitworkspace.Repository) (gitworkspace.BatchResult, error) {
		outcome := gitworkspace.OutcomeSkippedDirty
		if repositories[0].Name == "behind" {
			outcome = gitworkspace.OutcomeUpdated
		}
		return gitworkspace.BatchResult{Results: []gitworkspace.OperationResult{{
			Repository: repositories[0], Outcome: outcome, Message: "secret path and stderr",
		}}}, nil
	}
	service := newGitJobTestService(t, provider)
	_, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{
		{Name: "dirty", RepositoryPath: `C:\dirty`}, {Name: "behind", RepositoryPath: `C:\behind`},
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartGitWorkspacePullContext(context.Background(), GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForGitWorkspaceJob(t, service, job.ID)
	if finished.Skipped != 1 || finished.Succeeded != 1 || finished.Failed != 0 {
		t.Fatalf("pull safety outcomes = %#v", finished)
	}
	for _, result := range finished.Outcomes {
		if strings.Contains(result.Message, "secret") {
			t.Fatalf("provider outcome message leaked: %#v", finished.Outcomes)
		}
	}
}

func TestGitWorkspaceProviderFailureIsSafeAndTyped(t *testing.T) {
	provider := &fakeGitWorkspaceProvider{}
	provider.history = func(context.Context, gitworkspace.Repository, int) ([]gitworkspace.Commit, error) {
		return []gitworkspace.Commit{}, errors.New("C:\\secret\\repo: fatal: token")
	}
	service := newGitJobTestService(t, provider)
	repositories, err := service.UpsertGitRepositoriesContext(context.Background(), []GitRepositoryUpsert{{Name: "repo", RepositoryPath: `C:\repo`}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadGitRepositoryHistoryContext(context.Background(), repositories[0].ID, 10)
	if err == nil || strings.Contains(err.Error(), "token") {
		t.Fatalf("history provider error = %v", err)
	}
	var operational *GitWorkspaceOperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("history error = %v, want GitWorkspaceOperationalError", err)
	}
	if _, err := service.UpdateGitWorkspaceSettingsContext(context.Background(), GitWorkspaceSettingsUpdateRequest{WorkerCount: 65, StaleDays: 30, ExpectedRevision: 1}); err == nil {
		t.Fatal("invalid worker count unexpectedly accepted")
	}
}
