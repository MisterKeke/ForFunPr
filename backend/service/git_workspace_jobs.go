package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"something/backend/gitworkspace"
)

// GitWorkspaceProvider is the service-owned dependency boundary for
// GitWorkspaceFun. Keeping the interface here makes jobs deterministic in
// service tests and keeps provider details out of the domain layer.
type GitWorkspaceProvider = gitworkspace.Provider

// The adapter package uses structurally equivalent models. These aliases are
// defined by the service so callers never need to import the adapter package.
// They are intentionally small and exclude all executable/path diagnostics.
type GitWorkspaceEnvironment = gitworkspace.EnvironmentCapability
type GitWorkspaceScanOptions = gitworkspace.ScanOptions
type GitWorkspaceProviderRepository = gitworkspace.Repository
type GitWorkspaceProviderStatus = gitworkspace.Status
type GitWorkspaceProviderCommit = gitworkspace.Commit
type GitWorkspaceProviderOperationOutcome = gitworkspace.OperationResult
type GitWorkspaceProviderScanResult = gitworkspace.ScanResult
type GitWorkspaceBatchResult = gitworkspace.BatchResult
type GitWorkspaceSynchronizationResult = gitworkspace.SynchronizationResult

// GitWorkspaceRepositorySummary is the inventory row used by dashboard
// callers. RepositoryPath remains private on the embedded domain model.
type GitWorkspaceRepositorySummary struct {
	Repository   GitRepository        `json:"repository"`
	CachedStatus *GitRepositoryStatus `json:"cached_status,omitempty"`
}

type GitWorkspaceRepositoryDetails struct {
	Repository   GitRepository        `json:"repository"`
	CachedStatus *GitRepositoryStatus `json:"cached_status,omitempty"`
	LiveStatus   *GitRepositoryStatus `json:"live_status,omitempty"`
	History      []GitWorkspaceCommit `json:"history"`
}

type GitWorkspaceCommit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

type GitWorkspaceStatusView struct {
	RepositoryID int                  `json:"repository_id"`
	Cached       *GitRepositoryStatus `json:"cached,omitempty"`
	Live         *GitRepositoryStatus `json:"live,omitempty"`
}

type GitWorkspaceDashboardTotals struct {
	TotalRepositories      int `json:"total_repositories"`
	MissingRepositories    int `json:"missing_repositories"`
	CleanRepositories      int `json:"clean_repositories"`
	DirtyRepositories      int `json:"dirty_repositories"`
	UnknownRepositories    int `json:"unknown_repositories"`
	AheadRepositories      int `json:"ahead_repositories"`
	BehindRepositories     int `json:"behind_repositories"`
	DivergedRepositories   int `json:"diverged_repositories"`
	NoUpstreamRepositories int `json:"no_upstream_repositories"`
}

type GitWorkspaceDashboard struct {
	Totals       GitWorkspaceDashboardTotals     `json:"totals"`
	Repositories []GitWorkspaceRepositorySummary `json:"repositories"`
	ReadAt       string                          `json:"read_at"`
}

type GitWorkspaceJobKind string

const (
	GitWorkspaceJobScan          GitWorkspaceJobKind = "scan"
	GitWorkspaceJobStatusRefresh GitWorkspaceJobKind = "status_refresh"
	GitWorkspaceJobFetch         GitWorkspaceJobKind = "fetch"
	GitWorkspaceJobPull          GitWorkspaceJobKind = "pull"
	GitWorkspaceJobSync          GitWorkspaceJobKind = "sync"
)

type GitWorkspaceJobState string

const (
	GitWorkspaceJobQueued    GitWorkspaceJobState = "queued"
	GitWorkspaceJobRunning   GitWorkspaceJobState = "running"
	GitWorkspaceJobComplete  GitWorkspaceJobState = "complete"
	GitWorkspaceJobFailed    GitWorkspaceJobState = "failed"
	GitWorkspaceJobCancelled GitWorkspaceJobState = "cancelled"
)

type GitWorkspaceOperationOutcome struct {
	RepositoryID   int    `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	Phase          string `json:"phase,omitempty"`
	Outcome        string `json:"outcome"`
	Message        string `json:"message,omitempty"`
	Error          string `json:"error,omitempty"`
	FetchOutcome   string `json:"fetch_outcome,omitempty"`
	PullOutcome    string `json:"pull_outcome,omitempty"`
}

type GitWorkspaceJob struct {
	ID          string                         `json:"id"`
	Kind        GitWorkspaceJobKind            `json:"kind"`
	State       GitWorkspaceJobState           `json:"state"`
	WorkspaceID int                            `json:"workspace_id,omitempty"`
	Attempted   int                            `json:"attempted"`
	Completed   int                            `json:"completed"`
	Succeeded   int                            `json:"succeeded"`
	Skipped     int                            `json:"skipped"`
	Failed      int                            `json:"failed"`
	Outcomes    []GitWorkspaceOperationOutcome `json:"outcomes"`
	CreatedAt   string                         `json:"created_at"`
	StartedAt   string                         `json:"started_at,omitempty"`
	CompletedAt string                         `json:"completed_at,omitempty"`
	Error       string                         `json:"error,omitempty"`
}

type GitWorkspaceJobRequest struct {
	WorkspaceID   int                     `json:"workspace_id,omitempty"`
	RepositoryIDs []int                   `json:"repository_ids,omitempty"`
	Filter        GitRepositoryListFilter `json:"filter,omitempty"`
}

type gitWorkspaceJobRecord struct {
	job             GitWorkspaceJob
	cancel          context.CancelFunc
	key             string
	cancelRequested bool
}

func (a *Service) detectGitWorkspaceCapability(ctx context.Context) {
	if a == nil {
		return
	}
	provider := a.gitWorkspaceProvider
	if provider == nil {
		a.SetCapability("git_workspaces", false, false, true, "Git workspace integration is unavailable.")
		a.lifecycleMu.Lock()
		a.gitWorkspaceCapabilityChecked = true
		a.lifecycleMu.Unlock()
		return
	}
	capability, err := provider.Detect(ctx)
	if err != nil || !capability.Available {
		warning := capability.Warning
		if warning == "" {
			warning = "Git workspace integration is unavailable."
		}
		a.SetCapability("git_workspaces", false, false, true, warning)
	} else {
		a.SetCapability("git_workspaces", true, false, false, "")
	}
	a.lifecycleMu.Lock()
	a.gitWorkspaceCapabilityChecked = true
	a.lifecycleMu.Unlock()
}

// SetGitWorkspaceProvider is intended for embedding and deterministic tests.
// It must be called before Startup; jobs already in flight retain their
// provider and are cancelled by the normal service lifecycle.
func (a *Service) SetGitWorkspaceProvider(provider GitWorkspaceProvider) {
	if a == nil {
		return
	}
	a.lifecycleMu.Lock()
	a.gitWorkspaceProvider = provider
	a.lifecycleMu.Unlock()
}

func (a *Service) gitWorkspaceProviderOrError() (GitWorkspaceProvider, error) {
	if a == nil {
		return nil, ErrBackendNotReady
	}
	a.lifecycleMu.Lock()
	provider := a.gitWorkspaceProvider
	checked := a.gitWorkspaceCapabilityChecked
	capability := a.capabilities["git_workspaces"]
	a.lifecycleMu.Unlock()
	if provider == nil || (checked && !capability.Available) {
		return nil, &GitWorkspaceOperationalError{Message: "Git workspace integration is unavailable."}
	}
	return provider, nil
}

func (a *Service) ListGitWorkspaceRepositoriesContext(ctx context.Context, filter GitRepositoryListFilter) ([]GitRepository, error) {
	return a.ListGitRepositoriesContext(ctx, filter)
}

func (a *Service) ListGitWorkspaceRepositorySummariesContext(ctx context.Context, filter GitRepositoryListFilter) ([]GitWorkspaceRepositorySummary, error) {
	repositories, err := a.ListGitRepositoriesContext(ctx, filter)
	if err != nil {
		return []GitWorkspaceRepositorySummary{}, err
	}
	result := make([]GitWorkspaceRepositorySummary, 0, len(repositories))
	for _, repository := range repositories {
		summary := GitWorkspaceRepositorySummary{Repository: repository}
		if status, statusErr := a.ReadGitRepositoryStatusContext(ctx, repository.ID); statusErr == nil {
			summary.CachedStatus = &status
		} else if !isGitWorkspaceNotFound(statusErr) {
			return []GitWorkspaceRepositorySummary{}, statusErr
		}
		result = append(result, summary)
	}
	return result, nil
}

func (a *Service) AddGitWorkspaceRootContext(ctx context.Context, request GitWorkspaceRootCreateRequest) (GitWorkspaceRoot, error) {
	return a.CreateGitWorkspaceRootContext(ctx, request)
}

func (a *Service) RemoveGitWorkspaceRootContext(ctx context.Context, request GitWorkspaceRootDeleteRequest) error {
	return a.DeleteGitWorkspaceRootContext(ctx, request)
}

func (a *Service) PruneMissingGitWorkspaceRepositoriesContext(ctx context.Context) (int, error) {
	return a.PruneUnreferencedMissingGitRepositoriesContext(ctx)
}

func (a *Service) ReadGitWorkspaceDashboardTotalsContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceDashboardTotals, error) {
	repositories, err := a.ListGitRepositoriesContext(ctx, filter)
	if err != nil {
		return GitWorkspaceDashboardTotals{}, err
	}
	totals := GitWorkspaceDashboardTotals{TotalRepositories: len(repositories)}
	for _, repository := range repositories {
		if repository.Missing {
			totals.MissingRepositories++
		}
		status, err := loadGitRepositoryStatusContext(ctx, a.db, repository.ID)
		if isGitWorkspaceNotFound(err) {
			totals.UnknownRepositories++
			continue
		}
		if err != nil {
			return GitWorkspaceDashboardTotals{}, err
		}
		switch {
		case status.Dirty:
			totals.DirtyRepositories++
		default:
			totals.CleanRepositories++
		}
		switch strings.ToLower(status.SyncState) {
		case "ahead":
			totals.AheadRepositories++
		case "behind":
			totals.BehindRepositories++
		case "diverged":
			totals.DivergedRepositories++
		case "no-upstream":
			totals.NoUpstreamRepositories++
		}
	}
	return totals, nil
}

func (a *Service) ReadGitWorkspaceDashboardContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceDashboard, error) {
	repositories, err := a.ListGitRepositoriesContext(ctx, filter)
	if err != nil {
		return GitWorkspaceDashboard{}, err
	}
	summaries := make([]GitWorkspaceRepositorySummary, 0, len(repositories))
	for _, repository := range repositories {
		summary := GitWorkspaceRepositorySummary{Repository: repository}
		if status, statusErr := loadGitRepositoryStatusContext(ctx, a.db, repository.ID); statusErr == nil {
			summary.CachedStatus = &status
		} else if !isGitWorkspaceNotFound(statusErr) {
			return GitWorkspaceDashboard{}, statusErr
		}
		summaries = append(summaries, summary)
	}
	totals, err := a.ReadGitWorkspaceDashboardTotalsContext(ctx, filter)
	if err != nil {
		return GitWorkspaceDashboard{}, err
	}
	return GitWorkspaceDashboard{Totals: totals, Repositories: summaries, ReadAt: a.now().UTC().Format(time.RFC3339Nano)}, nil
}

func (a *Service) ReadGitWorkspaceCachedDashboardContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceDashboard, error) {
	return a.ReadGitWorkspaceDashboardContext(ctx, filter)
}

func (a *Service) StartGitWorkspaceStatusContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceStatusRefreshContext(ctx, filter)
}

func (a *Service) StartGitWorkspaceLocalStatusRefreshContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceStatusRefreshContext(ctx, filter)
}

func (a *Service) ReadGitRepositoryLiveStatusContext(ctx context.Context, repositoryID int) (GitRepositoryStatus, error) {
	repository, err := a.GetGitRepositoryContext(ctx, repositoryID)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	provider, err := a.gitWorkspaceProviderOrError()
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	status, err := provider.Status(ctx, GitWorkspaceProviderRepository{Name: repository.Name, Path: repository.RepositoryPath})
	if err != nil {
		return GitRepositoryStatus{}, safeGitWorkspaceProviderError("read repository status", err)
	}
	return a.writeProviderStatus(ctx, repository, status)
}

func (a *Service) ReadGitRepositoryStatusViewContext(ctx context.Context, repositoryID int, live bool) (GitWorkspaceStatusView, error) {
	if err := validateGitRepositoryID(repositoryID); err != nil {
		return GitWorkspaceStatusView{}, err
	}
	view := GitWorkspaceStatusView{RepositoryID: repositoryID}
	if cached, err := a.ReadGitRepositoryStatusContext(ctx, repositoryID); err == nil {
		view.Cached = &cached
	} else if !isGitWorkspaceNotFound(err) {
		return GitWorkspaceStatusView{}, err
	}
	if live {
		current, err := a.ReadGitRepositoryLiveStatusContext(ctx, repositoryID)
		if err != nil {
			return GitWorkspaceStatusView{}, err
		}
		view.Live = &current
	}
	return view, nil
}

func (a *Service) ReadGitRepositoryHistoryContext(ctx context.Context, repositoryID, limit int) ([]GitWorkspaceCommit, error) {
	if err := validateGitRepositoryID(repositoryID); err != nil {
		return []GitWorkspaceCommit{}, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		return []GitWorkspaceCommit{}, &ValidationError{Field: "limit", Message: "history limit must be between 1 and 500"}
	}
	repository, err := a.GetGitRepositoryContext(ctx, repositoryID)
	if err != nil {
		return []GitWorkspaceCommit{}, err
	}
	provider, err := a.gitWorkspaceProviderOrError()
	if err != nil {
		return []GitWorkspaceCommit{}, err
	}
	commits, err := provider.History(ctx, GitWorkspaceProviderRepository{Name: repository.Name, Path: repository.RepositoryPath}, limit)
	if err != nil {
		return mapGitWorkspaceCommits(commits), safeGitWorkspaceProviderError("read repository history", err)
	}
	return mapGitWorkspaceCommits(commits), nil
}

func (a *Service) GetGitRepositoryDetailsWithOptionsContext(ctx context.Context, repositoryID int, live bool, historyLimit int) (GitWorkspaceRepositoryDetails, error) {
	repository, err := a.GetGitRepositoryContext(ctx, repositoryID)
	if err != nil {
		return GitWorkspaceRepositoryDetails{}, err
	}
	details := GitWorkspaceRepositoryDetails{Repository: repository, History: []GitWorkspaceCommit{}}
	if status, statusErr := a.ReadGitRepositoryStatusContext(ctx, repositoryID); statusErr == nil {
		details.CachedStatus = &status
	} else if !isGitWorkspaceNotFound(statusErr) {
		return GitWorkspaceRepositoryDetails{}, statusErr
	}
	if live {
		status, err := a.ReadGitRepositoryLiveStatusContext(ctx, repositoryID)
		if err != nil {
			return GitWorkspaceRepositoryDetails{}, err
		}
		details.LiveStatus = &status
	}
	if historyLimit != 0 {
		history, err := a.ReadGitRepositoryHistoryContext(ctx, repositoryID, historyLimit)
		if err != nil {
			return GitWorkspaceRepositoryDetails{}, err
		}
		details.History = history
	}
	return details, nil
}

func (a *Service) GetGitRepositoryDetailsContext(ctx context.Context, repositoryID int) (GitWorkspaceRepositoryDetails, error) {
	return a.GetGitRepositoryDetailsWithOptionsContext(ctx, repositoryID, false, 0)
}

func (a *Service) ReadGitRepositoryDetailsContext(ctx context.Context, repositoryID int) (GitWorkspaceRepositoryDetails, error) {
	return a.GetGitRepositoryDetailsContext(ctx, repositoryID)
}

func (a *Service) GetGitRepositoryHistoryContext(ctx context.Context, repositoryID, limit int) ([]GitWorkspaceCommit, error) {
	return a.ReadGitRepositoryHistoryContext(ctx, repositoryID, limit)
}

func (a *Service) StartGitWorkspaceScanContext(ctx context.Context, workspaceID int) (GitWorkspaceJob, error) {
	if err := validateGitWorkspaceRootID(workspaceID); err != nil {
		return GitWorkspaceJob{}, err
	}
	root, err := a.GetGitWorkspaceRootContext(ctx, workspaceID)
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	provider, err := a.gitWorkspaceProviderOrError()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	return a.startGitWorkspaceJob(ctx, GitWorkspaceJobScan, fmt.Sprintf("scan:%d", workspaceID), workspaceID, true,
		func(jobCtx context.Context, jobID string) error {
			result, scanErr := provider.Scan(jobCtx, root.RootPath)
			if jobCtx.Err() != nil {
				return jobCtx.Err()
			}
			if scanErr != nil {
				// A provider may return partial discoveries with an error. Do not
				// replace a known-good inventory with that incomplete result.
				return safeGitWorkspaceProviderError("scan repositories", scanErr)
			}
			repositories := make([]GitRepositoryScanResult, 0, len(result.Repositories))
			for _, repository := range result.Repositories {
				repositories = append(repositories, GitRepositoryScanResult{Name: repository.Name, RepositoryPath: repository.Path})
			}
			if err := a.ReconcileGitWorkspaceScanContext(jobCtx, GitWorkspaceScanReconcileRequest{WorkspaceID: workspaceID, Repositories: repositories}); err != nil {
				return err
			}
			a.emitGitWorkspaceInventoryChanged()
			for _, repository := range result.Repositories {
				repo, loadErr := a.findGitRepositoryByPathContext(jobCtx, repository.Path)
				if loadErr == nil {
					a.recordGitWorkspaceOutcome(jobID, GitWorkspaceOperationOutcome{RepositoryID: repo.ID, RepositoryName: repo.Name, Outcome: "discovered"})
				}
			}
			return nil
		})
}

func (a *Service) StartGitWorkspaceRescanContext(ctx context.Context, workspaceID int) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceScanContext(ctx, workspaceID)
}

func (a *Service) StartGitWorkspaceRescanJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceScanContext(ctx, request.WorkspaceID)
}

func (a *Service) StartGitWorkspaceScanJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceScanContext(ctx, request.WorkspaceID)
}

func (a *Service) StartGitWorkspaceStatusRefreshContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.startRepositoryGitWorkspaceJob(ctx, GitWorkspaceJobStatusRefresh, filter, func(jobCtx context.Context, repository GitRepository) (GitWorkspaceOperationOutcome, error) {
		provider, err := a.gitWorkspaceProviderOrError()
		if err != nil {
			return failedGitWorkspaceOutcome(repository, "status", err), nil
		}
		status, err := provider.Status(jobCtx, GitWorkspaceProviderRepository{Name: repository.Name, Path: repository.RepositoryPath})
		if err != nil {
			return failedGitWorkspaceOutcome(repository, "status", safeGitWorkspaceProviderError("refresh repository status", err)), nil
		}
		if err := jobCtx.Err(); err != nil {
			return GitWorkspaceOperationOutcome{
				RepositoryID: repository.ID, RepositoryName: repository.Name,
				Phase: "status", Outcome: "cancelled", Error: safeGitWorkspaceError(err),
			}, nil
		}
		if _, err := a.writeProviderStatus(jobCtx, repository, status); err != nil {
			return failedGitWorkspaceOutcome(repository, "status", err), nil
		}
		a.emitGitWorkspaceStatusUpdated(repository.ID)
		return GitWorkspaceOperationOutcome{RepositoryID: repository.ID, RepositoryName: repository.Name, Phase: "status", Outcome: "updated"}, nil
	})
}

func (a *Service) StartGitWorkspaceStatusRefreshJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceStatusRefreshContext(ctx, gitWorkspaceJobFilter(request))
}

func (a *Service) StartGitWorkspaceFetchContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.startRepositoryGitWorkspaceJob(ctx, GitWorkspaceJobFetch, filter, func(jobCtx context.Context, repository GitRepository) (GitWorkspaceOperationOutcome, error) {
		provider, err := a.gitWorkspaceProviderOrError()
		if err != nil {
			return failedGitWorkspaceOutcome(repository, "fetch", err), nil
		}
		result, providerErr := provider.Fetch(jobCtx, []GitWorkspaceProviderRepository{{Name: repository.Name, Path: repository.RepositoryPath}})
		if len(result.Results) > 0 {
			return mapGitWorkspaceOutcome(repository, "fetch", result.Results[0]), nil
		}
		if providerErr != nil {
			return failedGitWorkspaceOutcome(repository, "fetch", safeGitWorkspaceProviderError("fetch repository", providerErr)), nil
		}
		return failedGitWorkspaceOutcome(repository, "fetch", &GitWorkspaceOperationalError{Message: "The Git workspace operation could not be completed."}), nil
	})
}

func (a *Service) StartGitWorkspacePullContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.startRepositoryGitWorkspaceJob(ctx, GitWorkspaceJobPull, filter, func(jobCtx context.Context, repository GitRepository) (GitWorkspaceOperationOutcome, error) {
		provider, err := a.gitWorkspaceProviderOrError()
		if err != nil {
			return failedGitWorkspaceOutcome(repository, "pull", err), nil
		}
		result, providerErr := provider.Pull(jobCtx, []GitWorkspaceProviderRepository{{Name: repository.Name, Path: repository.RepositoryPath}})
		if len(result.Results) > 0 {
			return mapGitWorkspaceOutcome(repository, "pull", result.Results[0]), nil
		}
		if providerErr != nil {
			return failedGitWorkspaceOutcome(repository, "pull", safeGitWorkspaceProviderError("pull repository", providerErr)), nil
		}
		return failedGitWorkspaceOutcome(repository, "pull", &GitWorkspaceOperationalError{Message: "The Git workspace operation could not be completed."}), nil
	})
}

func (a *Service) StartGitWorkspaceSyncContext(ctx context.Context, filter GitRepositoryListFilter) (GitWorkspaceJob, error) {
	return a.startRepositoryGitWorkspaceJob(ctx, GitWorkspaceJobSync, filter, func(jobCtx context.Context, repository GitRepository) (GitWorkspaceOperationOutcome, error) {
		provider, err := a.gitWorkspaceProviderOrError()
		if err != nil {
			return failedGitWorkspaceOutcome(repository, "sync", err), nil
		}
		result, providerErr := provider.Sync(jobCtx, []GitWorkspaceProviderRepository{{Name: repository.Name, Path: repository.RepositoryPath}})
		outcome := GitWorkspaceOperationOutcome{RepositoryID: repository.ID, RepositoryName: repository.Name, Phase: "sync"}
		if len(result.Fetch) > 0 {
			outcome.FetchOutcome = string(result.Fetch[0].Outcome)
			mapped := mapGitWorkspaceOutcome(repository, "fetch", result.Fetch[0])
			outcome.Message = mapped.Message
			outcome.Error = mapped.Error
		}
		if len(result.Pull) > 0 {
			outcome.PullOutcome = string(result.Pull[0].Outcome)
			mapped := mapGitWorkspaceOutcome(repository, "pull", result.Pull[0])
			if mapped.Message != "" {
				outcome.Message = mapped.Message
			}
			if mapped.Error != "" {
				outcome.Error = mapped.Error
			}
		}
		if providerErr != nil && outcome.FetchOutcome == "" && outcome.PullOutcome == "" {
			return failedGitWorkspaceOutcome(repository, "sync", safeGitWorkspaceProviderError("synchronize repository", providerErr)), nil
		}
		outcome.Outcome = outcome.PullOutcome
		if outcome.Outcome == "" {
			outcome.Outcome = outcome.FetchOutcome
		}
		if outcome.Outcome == "" {
			outcome.Outcome = "failed"
			outcome.Error = "The Git workspace operation could not be completed."
		}
		if providerErr != nil && outcome.FetchOutcome == "" && outcome.PullOutcome == "" {
			outcome.Error = safeGitWorkspaceProviderError("synchronize repository", providerErr).Error()
		}
		return outcome, nil
	})
}

func (a *Service) StartGitWorkspaceFetchJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceFetchContext(ctx, gitWorkspaceJobFilter(request))
}

func (a *Service) StartGitWorkspacePullJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspacePullContext(ctx, gitWorkspaceJobFilter(request))
}

func (a *Service) StartGitWorkspaceSyncJobContext(ctx context.Context, request GitWorkspaceJobRequest) (GitWorkspaceJob, error) {
	return a.StartGitWorkspaceSyncContext(ctx, gitWorkspaceJobFilter(request))
}

func gitWorkspaceJobFilter(request GitWorkspaceJobRequest) GitRepositoryListFilter {
	filter := request.Filter
	if len(request.RepositoryIDs) > 0 {
		filter.RepositoryIDs = append([]int{}, request.RepositoryIDs...)
	}
	if request.WorkspaceID != 0 {
		filter.WorkspaceID = request.WorkspaceID
	}
	return filter
}

func (a *Service) startRepositoryGitWorkspaceJob(ctx context.Context, kind GitWorkspaceJobKind, filter GitRepositoryListFilter, work func(context.Context, GitRepository) (GitWorkspaceOperationOutcome, error)) (GitWorkspaceJob, error) {
	repositories, err := a.ListGitRepositoriesContext(ctx, filter)
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	provider, err := a.gitWorkspaceProviderOrError()
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	_ = provider
	return a.startGitWorkspaceJob(ctx, kind, string(kind), filter.WorkspaceID, kind != GitWorkspaceJobStatusRefresh,
		func(jobCtx context.Context, jobID string) error {
			settings, err := a.GetGitWorkspaceSettingsContext(jobCtx)
			if err != nil {
				return err
			}
			runGitWorkspaceRepositoryWorkers(jobCtx, settings.WorkerCount, repositories, func(repository GitRepository) {
				outcome, workErr := work(jobCtx, repository)
				if jobCtx.Err() != nil {
					outcome = GitWorkspaceOperationOutcome{
						RepositoryID:   repository.ID,
						RepositoryName: repository.Name,
						Phase:          string(kind),
						Outcome:        "cancelled",
						Error:          safeGitWorkspaceError(jobCtx.Err()),
					}
				} else if workErr != nil {
					outcome = failedGitWorkspaceOutcome(repository, string(kind), workErr)
				}
				a.recordGitWorkspaceOutcome(jobID, outcome)
			})
			return nil
		})
}

func (a *Service) startGitWorkspaceJob(ctx context.Context, kind GitWorkspaceJobKind, key string, workspaceID int, mutation bool, run func(context.Context, string) error) (GitWorkspaceJob, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return GitWorkspaceJob{}, err
	}
	operationContext, done, err := a.BeginOperation(a.OperationContext())
	if err != nil {
		return GitWorkspaceJob{}, err
	}
	jobID := uuid.NewString()
	now := a.now().UTC().Format(time.RFC3339Nano)
	record := &gitWorkspaceJobRecord{job: GitWorkspaceJob{ID: jobID, Kind: kind, State: GitWorkspaceJobQueued, WorkspaceID: workspaceID, Outcomes: []GitWorkspaceOperationOutcome{}, CreatedAt: now}, key: key}
	jobContext, cancel := context.WithCancel(operationContext)
	record.cancel = cancel

	a.gitWorkspaceMu.Lock()
	if a.gitWorkspaceJobs == nil {
		a.gitWorkspaceJobs = make(map[string]*gitWorkspaceJobRecord)
	}
	if a.gitWorkspaceActiveKeys == nil {
		a.gitWorkspaceActiveKeys = make(map[string]string)
	}
	if activeID := a.gitWorkspaceActiveKeys[key]; activeID != "" {
		a.gitWorkspaceMu.Unlock()
		cancel()
		done()
		return GitWorkspaceJob{}, &ConflictError{Resource: "Git workspace job", Message: "An equivalent Git workspace job is already running."}
	}
	if mutation && a.gitWorkspaceMutationJob != "" {
		a.gitWorkspaceMu.Unlock()
		cancel()
		done()
		return GitWorkspaceJob{}, &ConflictError{Resource: "Git workspace job", Message: "A Git workspace mutation job is already running."}
	}
	a.gitWorkspaceJobs[jobID] = record
	a.gitWorkspaceActiveKeys[key] = jobID
	if mutation {
		a.gitWorkspaceMutationJob = jobID
	}
	snapshot := cloneGitWorkspaceJob(record.job)
	a.gitWorkspaceMu.Unlock()

	go a.runGitWorkspaceJob(jobContext, record, run, done)
	return snapshot, nil
}

func (a *Service) runGitWorkspaceJob(ctx context.Context, record *gitWorkspaceJobRecord, run func(context.Context, string) error, done func()) {
	defer done()
	a.gitWorkspaceMu.Lock()
	if !record.cancelRequested {
		record.job.State = GitWorkspaceJobRunning
		record.job.StartedAt = a.now().UTC().Format(time.RFC3339Nano)
	} else {
		record.job.State = GitWorkspaceJobCancelled
	}
	snapshot := cloneGitWorkspaceJob(record.job)
	a.gitWorkspaceMu.Unlock()
	a.emitGitWorkspaceJobProgress(snapshot)

	err := run(ctx, record.job.ID)
	a.gitWorkspaceMu.Lock()
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		record.job.State = GitWorkspaceJobCancelled
		record.job.Error = "The Git workspace job was canceled."
	case err != nil:
		record.job.State = GitWorkspaceJobFailed
		record.job.Error = safeGitWorkspaceError(err)
	default:
		record.job.State = GitWorkspaceJobComplete
	}
	record.job.CompletedAt = a.now().UTC().Format(time.RFC3339Nano)
	record.cancel = nil
	if a.gitWorkspaceActiveKeys[record.key] == record.job.ID {
		delete(a.gitWorkspaceActiveKeys, record.key)
	}
	if a.gitWorkspaceMutationJob == record.job.ID {
		a.gitWorkspaceMutationJob = ""
	}
	snapshot = cloneGitWorkspaceJob(record.job)
	a.gitWorkspaceMu.Unlock()
	a.emitGitWorkspaceJobComplete(snapshot)
}

func (a *Service) recordGitWorkspaceOutcome(jobID string, outcome GitWorkspaceOperationOutcome) {
	a.gitWorkspaceMu.Lock()
	record := a.gitWorkspaceJobs[jobID]
	if record == nil {
		a.gitWorkspaceMu.Unlock()
		return
	}
	record.job.Attempted++
	record.job.Completed++
	record.job.Outcomes = append(record.job.Outcomes, outcome)
	if isGitWorkspaceSkipped(outcome.Outcome) {
		record.job.Skipped++
	} else if outcome.Outcome == "failed" || outcome.Error != "" {
		record.job.Failed++
	} else if outcome.Outcome != "cancelled" && outcome.Outcome != "canceled" {
		record.job.Succeeded++
	}
	snapshot := cloneGitWorkspaceJob(record.job)
	a.gitWorkspaceMu.Unlock()
	a.emitGitWorkspaceJobProgress(snapshot)
}

func (a *Service) GetGitWorkspaceJobContext(ctx context.Context, id string) (GitWorkspaceJob, error) {
	if strings.TrimSpace(id) == "" {
		return GitWorkspaceJob{}, &ValidationError{Field: "id", Message: "job ID cannot be empty"}
	}
	a.gitWorkspaceMu.Lock()
	defer a.gitWorkspaceMu.Unlock()
	record := a.gitWorkspaceJobs[id]
	if record == nil {
		return GitWorkspaceJob{}, &NotFoundError{Resource: "Git workspace job", Key: id}
	}
	return cloneGitWorkspaceJob(record.job), nil
}

func (a *Service) ListGitWorkspaceJobsContext(ctx context.Context) ([]GitWorkspaceJob, error) {
	a.gitWorkspaceMu.Lock()
	defer a.gitWorkspaceMu.Unlock()
	jobs := make([]GitWorkspaceJob, 0, len(a.gitWorkspaceJobs))
	for _, record := range a.gitWorkspaceJobs {
		jobs = append(jobs, cloneGitWorkspaceJob(record.job))
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	return jobs, nil
}

func (a *Service) ReadGitWorkspaceJobContext(ctx context.Context, id string) (GitWorkspaceJob, error) {
	return a.GetGitWorkspaceJobContext(ctx, id)
}

func (a *Service) ReadGitWorkspaceJobsContext(ctx context.Context) ([]GitWorkspaceJob, error) {
	return a.ListGitWorkspaceJobsContext(ctx)
}

func (a *Service) CancelGitWorkspaceContext(ctx context.Context, id string) (GitWorkspaceJob, error) {
	return a.CancelGitWorkspaceJobContext(ctx, id)
}

func (a *Service) CancelGitWorkspaceJobContext(ctx context.Context, id string) (GitWorkspaceJob, error) {
	a.gitWorkspaceMu.Lock()
	record := a.gitWorkspaceJobs[id]
	if record == nil {
		a.gitWorkspaceMu.Unlock()
		return GitWorkspaceJob{}, &NotFoundError{Resource: "Git workspace job", Key: id}
	}
	snapshot := cloneGitWorkspaceJob(record.job)
	if record.cancel != nil && (record.job.State == GitWorkspaceJobQueued || record.job.State == GitWorkspaceJobRunning) {
		record.cancelRequested = true
		record.job.State = GitWorkspaceJobCancelled
		record.cancel()
		snapshot = cloneGitWorkspaceJob(record.job)
	}
	a.gitWorkspaceMu.Unlock()
	if snapshot.State == GitWorkspaceJobCancelled {
		a.emitGitWorkspaceJobProgress(snapshot)
	}
	return snapshot, nil
}

func (a *Service) CancelGitWorkspaceJobByIDContext(ctx context.Context, id string) (GitWorkspaceJob, error) {
	return a.CancelGitWorkspaceJobContext(ctx, id)
}

func runGitWorkspaceRepositoryWorkers(ctx context.Context, workers int, repositories []GitRepository, work func(GitRepository)) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(repositories) == 0 {
		return
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(repositories) {
		workers = len(repositories)
	}
	queue := make(chan GitRepository)
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for {
				var repository GitRepository
				var ok bool
				select {
				case <-ctx.Done():
					return
				case repository, ok = <-queue:
					if !ok {
						return
					}
				}
				if err := ctx.Err(); err != nil {
					return
				}
				work(repository)
			}
		}()
	}

feed:
	for _, repository := range repositories {
		select {
		case <-ctx.Done():
			break feed
		case queue <- repository:
		}
	}
	close(queue)
	wait.Wait()
}

func (a *Service) writeProviderStatus(ctx context.Context, repository GitRepository, status GitWorkspaceProviderStatus) (GitRepositoryStatus, error) {
	remoteDisplay, err := sanitizeGitRemoteDisplay(status.Remote)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	remoteWebURL, err := gitRemoteWebURLFromDisplay(remoteDisplay)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	return a.WriteGitRepositoryStatusContext(ctx, GitRepositoryStatusWriteRequest{
		RepositoryID:  repository.ID,
		Branch:        status.Branch.Name,
		Dirty:         status.Changes.Modified+status.Changes.Added+status.Changes.Deleted+status.Changes.Renamed+status.Changes.Untracked > 0,
		ModifiedCount: status.Changes.Modified, AddedCount: status.Changes.Added, DeletedCount: status.Changes.Deleted,
		RenamedCount: status.Changes.Renamed, UntrackedCount: status.Changes.Untracked,
		Upstream: status.Sync.Upstream, AheadCount: status.Sync.Ahead, BehindCount: status.Sync.Behind,
		SyncState: string(status.Sync.State), RemoteDisplay: remoteDisplay, RemoteWebURL: remoteWebURL,
		CheckedAt: a.now().UTC().Format(time.RFC3339Nano),
	})
}

func gitRemoteWebURLFromDisplay(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.Contains(value, "://") {
		value = "https://" + strings.TrimPrefix(value, "/")
	}
	return normalizeGitRemoteWebURL(value)
}

func (a *Service) findGitRepositoryByPathContext(ctx context.Context, path string) (GitRepository, error) {
	pathKey := strings.ToLower(path)
	rows, err := a.db.QueryContext(ctx, `SELECT id, name, repository_path, repository_path_key, missing, last_seen_at, created_at, updated_at FROM git_repositories WHERE repository_path_key = ?`, pathKey)
	if err != nil {
		return GitRepository{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return GitRepository{}, &NotFoundError{Resource: "Git repository"}
	}
	return scanGitRepository(rows)
}

func mapGitWorkspaceCommits(commits []GitWorkspaceProviderCommit) []GitWorkspaceCommit {
	result := make([]GitWorkspaceCommit, len(commits))
	for index, commit := range commits {
		result[index] = GitWorkspaceCommit{Hash: commit.Hash, Message: commit.Message, Author: commit.Author, Timestamp: commit.Timestamp}
	}
	return result
}

func mapGitWorkspaceOutcome(repository GitRepository, phase string, providerOutcome GitWorkspaceProviderOperationOutcome) GitWorkspaceOperationOutcome {
	errorMessage := ""
	if providerOutcome.Diagnostic != nil {
		errorMessage = safeGitWorkspaceError(providerOutcome.Diagnostic)
	}
	if providerOutcome.Outcome == "failed" && errorMessage == "" {
		errorMessage = "The Git workspace operation could not be completed."
	}
	message := safeGitWorkspaceOutcomeMessage(providerOutcome.Outcome)
	return GitWorkspaceOperationOutcome{RepositoryID: repository.ID, RepositoryName: repository.Name, Phase: phase, Outcome: string(providerOutcome.Outcome), Message: message, Error: errorMessage}
}

func safeGitWorkspaceOutcomeMessage(outcome gitworkspace.OperationOutcome) string {
	switch outcome {
	case gitworkspace.OutcomeUpdated:
		return "Repository updated."
	case gitworkspace.OutcomeSkippedDirty:
		return "Working tree contains changes."
	case gitworkspace.OutcomeSkippedNoUpstream:
		return "Repository has no upstream branch."
	case gitworkspace.OutcomeSkippedDiverged:
		return "Branch has diverged."
	case gitworkspace.OutcomeAlreadyUpToDate:
		return "Repository is already up to date."
	case gitworkspace.OutcomeCancelled:
		return "The Git workspace operation was canceled."
	default:
		return ""
	}
}

func failedGitWorkspaceOutcome(repository GitRepository, phase string, err error) GitWorkspaceOperationOutcome {
	return GitWorkspaceOperationOutcome{RepositoryID: repository.ID, RepositoryName: repository.Name, Phase: phase, Outcome: "failed", Error: safeGitWorkspaceError(err)}
}

func isGitWorkspaceSkipped(outcome string) bool {
	return strings.HasPrefix(outcome, "skipped") || outcome == "already_up_to_date" || outcome == "no-op"
}

func cloneGitWorkspaceJob(job GitWorkspaceJob) GitWorkspaceJob {
	job.Outcomes = append([]GitWorkspaceOperationOutcome{}, job.Outcomes...)
	return job
}

func safeGitWorkspaceError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "The Git workspace operation was canceled."
	}
	if typed, ok := err.(*GitWorkspaceOperationalError); ok && typed.Message != "" {
		return typed.Message
	}
	return "The Git workspace operation could not be completed."
}

func safeGitWorkspaceProviderError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return context.Canceled
	}
	var adapterErr *gitworkspace.AdapterError
	if errors.As(err, &adapterErr) {
		switch adapterErr.Category {
		case gitworkspace.ErrorInvalidInput:
			return &ValidationError{Field: "git_workspace", Message: "The Git workspace request is invalid."}
		case gitworkspace.ErrorUnavailable:
			return &GitWorkspaceOperationalError{Operation: operation, Message: "Git is not available on this system.", cause: err}
		case gitworkspace.ErrorUnsupported:
			return &GitWorkspaceOperationalError{Operation: operation, Message: "The repository operation is unsupported.", cause: err}
		}
	}
	return &GitWorkspaceOperationalError{Operation: operation, Message: "The Git workspace operation could not be completed.", cause: err}
}

func isGitWorkspaceNotFound(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}
