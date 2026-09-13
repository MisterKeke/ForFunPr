package gitworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	workspace "github.com/MisterKeke/GitWorkspaceFun/workspace"
)

type fakeManager struct {
	detectEnvironment workspace.Environment
	detectErr         error
	scanRepositories  []workspace.Repository
	scanErr           error
	status            workspace.RepositoryStatus
	statusErr         error
	history           []workspace.Commit
	historyErr        error
	fetch             []workspace.RepositoryOutcome
	fetchErr          error
	pull              []workspace.RepositoryOutcome
	pullErr           error
	sync              workspace.SyncResult
	syncErr           error
	seenContext       context.Context
}

func (f *fakeManager) DetectGit(ctx context.Context) (workspace.Environment, error) {
	f.seenContext = ctx
	return f.detectEnvironment, f.detectErr
}

func (f *fakeManager) Scan(ctx context.Context, _ string, _ ...workspace.ScanOptions) ([]workspace.Repository, error) {
	f.seenContext = ctx
	return f.scanRepositories, f.scanErr
}

func (f *fakeManager) Status(ctx context.Context, _ workspace.Repository) (workspace.RepositoryStatus, error) {
	f.seenContext = ctx
	return f.status, f.statusErr
}

func (f *fakeManager) History(ctx context.Context, _ workspace.Repository, _ int) ([]workspace.Commit, error) {
	f.seenContext = ctx
	return f.history, f.historyErr
}

func (f *fakeManager) Fetch(ctx context.Context, _ []workspace.Repository) ([]workspace.RepositoryOutcome, error) {
	f.seenContext = ctx
	return f.fetch, f.fetchErr
}

func (f *fakeManager) Pull(ctx context.Context, _ []workspace.Repository) ([]workspace.RepositoryOutcome, error) {
	f.seenContext = ctx
	return f.pull, f.pullErr
}

func (f *fakeManager) Sync(ctx context.Context, _ []workspace.Repository) (workspace.SyncResult, error) {
	f.seenContext = ctx
	return f.sync, f.syncErr
}

func newFakeAdapter(fake *fakeManager) *Adapter {
	return &Adapter{manager: fake}
}

func TestStatusMapsWorkspaceModels(t *testing.T) {
	stamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	fake := &fakeManager{status: workspace.RepositoryStatus{
		Repository: workspace.Repository{Name: "Something", Path: `D:\Projects\Something`},
		Branch:     workspace.Branch{Name: "main", Detached: false, Commit: "abc"},
		Changes: workspace.Changes{
			Modified: 1, Added: 2, Deleted: 3, Renamed: 4, Untracked: 5,
			Lines: []string{" M main.go"},
		},
		Sync:   workspace.SyncStatus{Upstream: "origin/main", Ahead: 1, Behind: 2, State: workspace.SyncDiverged},
		Remote: "https://github.com/MisterKeke/ForFunPr",
	}}

	got, err := newFakeAdapter(fake).Status(context.Background(), Repository{Name: "Something", Path: `D:\Projects\Something`})
	if err != nil {
		t.Fatal(err)
	}
	want := Status{
		Repository: Repository{Name: "Something", Path: `D:\Projects\Something`},
		Branch:     Branch{Name: "main", Commit: "abc"},
		Changes:    ChangeCounts{Modified: 1, Added: 2, Deleted: 3, Renamed: 4, Untracked: 5, Lines: []string{" M main.go"}},
		Sync:       SyncStatus{Upstream: "origin/main", Ahead: 1, Behind: 2, State: SyncDiverged},
		Remote:     "https://github.com/MisterKeke/ForFunPr",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	fake.history = []workspace.Commit{{Hash: "abc", Message: "initial", Author: "A", Timestamp: stamp}}
	history, err := newFakeAdapter(fake).History(context.Background(), want.Repository, 5)
	if err != nil || !reflect.DeepEqual(history, []Commit{{Hash: "abc", Message: "initial", Author: "A", Timestamp: stamp}}) {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}

func TestEnvironmentAndSynchronizationModelsAreOwnedAndMapped(t *testing.T) {
	fake := &fakeManager{
		detectEnvironment: workspace.Environment{
			GitPath: `C:\Program Files\Git\bin\git.exe`,
			Version: "git version 2.49.0",
		},
		pull: []workspace.RepositoryOutcome{{
			Repository: workspace.Repository{Name: "dirty", Path: `D:\dirty`},
			Outcome:    workspace.OutcomeSkippedDirty,
			Message:    "working tree contains changes",
		}},
		sync: workspace.SyncResult{
			Fetch: []workspace.RepositoryOutcome{{
				Repository: workspace.Repository{Name: "repo", Path: `D:\repo`},
				Outcome:    workspace.OutcomeUpdated,
			}},
			Pull: []workspace.RepositoryOutcome{{
				Repository: workspace.Repository{Name: "repo", Path: `D:\repo`},
				Outcome:    workspace.OutcomeAlreadyUpToDate,
			}},
		},
	}
	adapter := newFakeAdapter(fake)

	capability, err := adapter.Detect(context.Background())
	if err != nil || !capability.Available || capability.Version != "git version 2.49.0" {
		t.Fatalf("capability=%+v err=%v", capability, err)
	}
	encoded, err := json.Marshal(capability)
	if err != nil || strings.Contains(string(encoded), fake.detectEnvironment.GitPath) {
		t.Fatalf("capability exposed executable path: %s", encoded)
	}

	pull, err := adapter.Pull(context.Background(), []Repository{{Name: "dirty", Path: `D:\dirty`}})
	if err != nil || len(pull.Results) != 1 || pull.Results[0].Outcome != OutcomeSkippedDirty {
		t.Fatalf("pull=%+v err=%v", pull, err)
	}
	syncResult, err := adapter.Sync(context.Background(), []Repository{{Name: "repo", Path: `D:\repo`}})
	if err != nil || len(syncResult.Fetch) != 1 || len(syncResult.Pull) != 1 || syncResult.Pull[0].Outcome != OutcomeAlreadyUpToDate {
		t.Fatalf("sync=%+v err=%v", syncResult, err)
	}
}

func TestScanAndFetchPreservePartialResults(t *testing.T) {
	sentinel := errors.New("remote token must not reach the UI")
	fake := &fakeManager{
		scanRepositories: []workspace.Repository{{Name: "one", Path: `D:\one`}},
		scanErr:          sentinel,
		fetch: []workspace.RepositoryOutcome{
			{Repository: workspace.Repository{Name: "good", Path: `D:\good`}, Outcome: workspace.OutcomeUpdated},
			{Repository: workspace.Repository{Name: "bad", Path: `D:\bad`}, Outcome: workspace.OutcomeFailed, Diagnostic: sentinel},
		},
		fetchErr: sentinel,
	}
	adapter := newFakeAdapter(fake)

	scan, scanErr := adapter.Scan(context.Background(), `D:\Projects`)
	if scanErr == nil || len(scan.Repositories) != 1 || scan.Error == "" || scan.Diagnostic != sentinel {
		t.Fatalf("scan=%+v err=%v", scan, scanErr)
	}
	if containsSecret(scan.Error, sentinel.Error()) {
		t.Fatalf("scan leaked diagnostic: %q", scan.Error)
	}

	batch, batchErr := adapter.Fetch(context.Background(), []Repository{{Name: "good"}, {Name: "bad"}})
	if batchErr == nil || len(batch.Results) != 2 || batch.Results[0].Outcome != OutcomeUpdated {
		t.Fatalf("batch=%+v err=%v", batch, batchErr)
	}
	if batch.Results[1].Error == "" || containsSecret(batch.Results[1].Error, sentinel.Error()) {
		t.Fatalf("unsafe per-repository error: %+v", batch.Results[1])
	}
	if !errors.Is(batch.Results[1].Diagnostic, sentinel) || !errors.Is(batchErr, sentinel) {
		t.Fatalf("diagnostic cause was not preserved: result=%v batch=%v", batch.Results[1].Diagnostic, batchErr)
	}
}

func TestCancellationIsPropagatedAndSafelyClassified(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeManager{scanErr: context.Canceled}
	result, err := newFakeAdapter(fake).Scan(ctx, `D:\Projects`)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v does not preserve cancellation", err)
	}
	adapterErr := new(AdapterError)
	if !errors.As(err, &adapterErr) || adapterErr.Category != ErrorCanceled {
		t.Fatalf("err=%v was not classified as canceled", err)
	}
	if fake.seenContext != ctx || result.Error != "The Git workspace operation was canceled." {
		t.Fatalf("context/result mismatch: same_context=%v result=%+v", fake.seenContext == ctx, result)
	}
}

func TestExternalErrorsBecomeStableSafeCategories(t *testing.T) {
	tests := []struct {
		name     string
		cause    error
		category ErrorCategory
		message  string
	}{
		{name: "git unavailable", cause: workspace.ErrGitNotFound, category: ErrorUnavailable, message: "Git is not available on this system."},
		{name: "invalid options", cause: workspace.ErrInvalidOptions, category: ErrorInvalidInput, message: "The Git workspace request is invalid."},
		{name: "unsupported remote", cause: workspace.ErrUnsupportedRemote, category: ErrorUnsupported, message: "The repository remote is unsupported."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := translateError("test", test.cause)
			if got.Category != test.category || got.Message != test.message || got.Error() != test.message {
				t.Fatalf("got category=%q message=%q", got.Category, got.Message)
			}
			if !errors.Is(got, test.cause) {
				t.Fatalf("translated error does not unwrap %v", test.cause)
			}
		})
	}
}

func containsSecret(value, secret string) bool {
	return len(secret) > 0 && strings.Contains(value, secret)
}
