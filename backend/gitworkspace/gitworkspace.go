// Package gitworkspace adapts GitWorkspaceFun's public workspace API to the
// narrow provider boundary owned by Something.
package gitworkspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	workspace "github.com/MisterKeke/GitWorkspaceFun/workspace"
)

// ErrorCategory is the stable classification exposed by the adapter.
type ErrorCategory string

const (
	ErrorCanceled     ErrorCategory = "canceled"
	ErrorInvalidInput ErrorCategory = "invalid_input"
	ErrorUnavailable  ErrorCategory = "unavailable"
	ErrorUnsupported  ErrorCategory = "unsupported"
	ErrorFailed       ErrorCategory = "failed"
)

// AdapterError is safe for presentation while retaining the underlying cause
// for local logging and errors.Is/errors.As checks.
type AdapterError struct {
	Category ErrorCategory `json:"category"`
	Message  string        `json:"message"`
	cause    error         `json:"-"`
}

func (e *AdapterError) Error() string {
	if e == nil {
		return "Git workspace operation failed."
	}
	return e.Message
}

func (e *AdapterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Repository identifies a local Git repository.
type Repository struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ChangeCounts describes the working-tree changes reported by Git.
type ChangeCounts struct {
	Modified  int      `json:"modified"`
	Added     int      `json:"added"`
	Deleted   int      `json:"deleted"`
	Renamed   int      `json:"renamed"`
	Untracked int      `json:"untracked"`
	Lines     []string `json:"lines"`
}

// Changes is a concise alias for ChangeCounts.
type Changes = ChangeCounts

// Branch describes the current branch or detached HEAD.
type Branch struct {
	Name     string `json:"name"`
	Detached bool   `json:"detached"`
	Commit   string `json:"commit,omitempty"`
}

// SyncState is the relationship between HEAD and its upstream.
type SyncState string

const (
	SyncClean      SyncState = "synced"
	SyncAhead      SyncState = "ahead"
	SyncBehind     SyncState = "behind"
	SyncDiverged   SyncState = "diverged"
	SyncNoUpstream SyncState = "no-upstream"
)

// SyncStatus describes upstream and ahead/behind state.
type SyncStatus struct {
	Upstream string    `json:"upstream,omitempty"`
	Ahead    int       `json:"ahead"`
	Behind   int       `json:"behind"`
	State    SyncState `json:"state"`
}

// Status is a structured snapshot of one repository.
type Status struct {
	Repository Repository   `json:"repository"`
	Branch     Branch       `json:"branch"`
	Changes    ChangeCounts `json:"changes"`
	Sync       SyncStatus   `json:"sync"`
	Remote     string       `json:"remote,omitempty"`
}

// RepositoryStatus is a descriptive alias for Status.
type RepositoryStatus = Status

// Commit is one recent repository commit.
type Commit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// ScanOptions controls one scan. A zero value uses the adapter default.
type ScanOptions struct {
	MaxResults int `json:"max_results,omitempty"`
}

// ScanResult preserves repositories discovered before a scan error.
type ScanResult struct {
	Repositories []Repository `json:"repositories"`
	Error        string       `json:"error,omitempty"`
	Diagnostic   error        `json:"-"`
}

// OperationOutcome classifies a per-repository fetch or pull operation.
type OperationOutcome string

const (
	OutcomeUpdated           OperationOutcome = "updated"
	OutcomeSkippedDirty      OperationOutcome = "skipped_dirty"
	OutcomeSkippedNoUpstream OperationOutcome = "skipped_no_upstream"
	OutcomeSkippedDiverged   OperationOutcome = "skipped_diverged"
	OutcomeAlreadyUpToDate   OperationOutcome = "already_up_to_date"
	OutcomeFailed            OperationOutcome = "failed"
	OutcomeCanceled          OperationOutcome = "cancelled"
	OutcomeCancelled         OperationOutcome = OutcomeCanceled
)

// OperationResult is the Something-owned representation of one repository
// operation. Diagnostic is deliberately excluded from JSON responses.
type OperationResult struct {
	Repository Repository       `json:"repository"`
	Outcome    OperationOutcome `json:"outcome"`
	Message    string           `json:"message,omitempty"`
	Error      string           `json:"error,omitempty"`
	Diagnostic error            `json:"-"`
}

// RepositoryOutcome is a descriptive alias for OperationResult.
type RepositoryOutcome = OperationResult

// Outcome is a concise alias for OperationOutcome.
type Outcome = OperationOutcome

// BatchResult preserves all per-repository results when one item fails.
type BatchResult struct {
	Results    []OperationResult `json:"results"`
	Error      string            `json:"error,omitempty"`
	Diagnostic error             `json:"-"`
}

// SynchronizationResult contains the fetch phase and safe-pull phase.
type SynchronizationResult struct {
	Fetch      []OperationResult `json:"fetch"`
	Pull       []OperationResult `json:"pull"`
	Error      string            `json:"error,omitempty"`
	Diagnostic error             `json:"-"`
}

// SyncResult is kept as a concise name for callers that prefer the operation
// name over the full result type.
type SyncResult = SynchronizationResult

// EnvironmentCapability describes whether Git is usable by Something. The
// executable path is intentionally not part of this model.
type EnvironmentCapability struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

// Environment is a concise alias for EnvironmentCapability.
type Environment = EnvironmentCapability

// Provider is the intentionally narrow Something-owned boundary for future
// service integration. It exposes finite workspace operations only.
type Provider interface {
	Detect(context.Context) (EnvironmentCapability, error)
	Scan(context.Context, string, ...ScanOptions) (ScanResult, error)
	Status(context.Context, Repository) (Status, error)
	History(context.Context, Repository, int) ([]Commit, error)
	Fetch(context.Context, []Repository) (BatchResult, error)
	Pull(context.Context, []Repository) (BatchResult, error)
	Sync(context.Context, []Repository) (SynchronizationResult, error)
}

type managerClient interface {
	DetectGit(context.Context) (workspace.Environment, error)
	Scan(context.Context, string, ...workspace.ScanOptions) ([]workspace.Repository, error)
	Status(context.Context, workspace.Repository) (workspace.RepositoryStatus, error)
	History(context.Context, workspace.Repository, int) ([]workspace.Commit, error)
	Fetch(context.Context, []workspace.Repository) ([]workspace.RepositoryOutcome, error)
	Pull(context.Context, []workspace.Repository) ([]workspace.RepositoryOutcome, error)
	Sync(context.Context, []workspace.Repository) (workspace.SyncResult, error)
}

// Options configures the underlying workspace manager without exposing its
// runner or any command execution seam.
type Options struct {
	Workers           int
	RepositoryTimeout time.Duration
	NetworkTimeout    time.Duration
	MaxHistory        int
	MaxScanResults    int
}

// Adapter is the production GitWorkspaceFun-backed provider.
type Adapter struct {
	manager managerClient
}

var _ Provider = (*Adapter)(nil)
var _ managerClient = (*workspace.Manager)(nil)

// New constructs a provider backed by GitWorkspaceFun's public manager.
func New(options Options) *Adapter {
	return &Adapter{manager: workspace.New(workspace.Options{
		Workers:           options.Workers,
		RepositoryTimeout: options.RepositoryTimeout,
		NetworkTimeout:    options.NetworkTimeout,
		MaxHistory:        options.MaxHistory,
		MaxScanResults:    options.MaxScanResults,
	})}
}

// NewProvider is an explicit interface-returning constructor for service
// wiring that should not depend on the concrete adapter.
func NewProvider(options Options) Provider {
	return New(options)
}

func (a *Adapter) Detect(ctx context.Context) (EnvironmentCapability, error) {
	ctx = normalizeContext(ctx)
	environment, err := a.manager.DetectGit(ctx)
	if err != nil {
		translated := translateError("detect Git", err)
		return EnvironmentCapability{Warning: translated.Error()}, translated
	}
	return EnvironmentCapability{Available: true, Version: environment.Version}, nil
}

// DetectGit is a descriptive alias for callers that use the library's naming.
func (a *Adapter) DetectGit(ctx context.Context) (EnvironmentCapability, error) {
	return a.Detect(ctx)
}

func (a *Adapter) Scan(ctx context.Context, root string, options ...ScanOptions) (ScanResult, error) {
	ctx = normalizeContext(ctx)
	externalOptions := make([]workspace.ScanOptions, 0, len(options))
	for _, option := range options {
		externalOptions = append(externalOptions, workspace.ScanOptions{MaxResults: option.MaxResults})
	}
	repositories, err := a.manager.Scan(ctx, root, externalOptions...)
	result := ScanResult{Repositories: mapRepositories(repositories)}
	if err != nil {
		translated := translateError("scan repositories", err)
		result.Error = translated.Error()
		result.Diagnostic = err
		return result, translated
	}
	return result, nil
}

func (a *Adapter) Status(ctx context.Context, repository Repository) (Status, error) {
	ctx = normalizeContext(ctx)
	status, err := a.manager.Status(ctx, toWorkspaceRepository(repository))
	if err != nil {
		return Status{}, translateError("inspect repository status", err)
	}
	return mapStatus(status), nil
}

// Inspect is a descriptive alias for status inspection.
func (a *Adapter) Inspect(ctx context.Context, repository Repository) (Status, error) {
	return a.Status(ctx, repository)
}

func (a *Adapter) History(ctx context.Context, repository Repository, limit int) ([]Commit, error) {
	ctx = normalizeContext(ctx)
	commits, err := a.manager.History(ctx, toWorkspaceRepository(repository), limit)
	result := mapCommits(commits)
	if err != nil {
		return result, translateError("read repository history", err)
	}
	return result, nil
}

func (a *Adapter) Fetch(ctx context.Context, repositories []Repository) (BatchResult, error) {
	ctx = normalizeContext(ctx)
	results, err := a.manager.Fetch(ctx, toWorkspaceRepositories(repositories))
	return mapBatchResult(results, err, "fetch repositories")
}

func (a *Adapter) Pull(ctx context.Context, repositories []Repository) (BatchResult, error) {
	ctx = normalizeContext(ctx)
	results, err := a.manager.Pull(ctx, toWorkspaceRepositories(repositories))
	return mapBatchResult(results, err, "pull repositories")
}

func (a *Adapter) Sync(ctx context.Context, repositories []Repository) (SynchronizationResult, error) {
	ctx = normalizeContext(ctx)
	result, err := a.manager.Sync(ctx, toWorkspaceRepositories(repositories))
	mapped := SynchronizationResult{
		Fetch: mapOperationResults(result.Fetch),
		Pull:  mapOperationResults(result.Pull),
	}
	if err != nil {
		mapped.Error = translateError("synchronize repositories", err).Error()
		mapped.Diagnostic = err
		return mapped, translateError("synchronize repositories", err)
	}
	return mapped, nil
}

// Synchronize is a descriptive alias for the fetch-plus-safe-pull operation.
func (a *Adapter) Synchronize(ctx context.Context, repositories []Repository) (SynchronizationResult, error) {
	return a.Sync(ctx, repositories)
}

func mapBatchResult(results []workspace.RepositoryOutcome, err error, operation string) (BatchResult, error) {
	mapped := BatchResult{Results: mapOperationResults(results)}
	if err != nil {
		translated := translateError(operation, err)
		mapped.Error = translated.Error()
		mapped.Diagnostic = err
		return mapped, translated
	}
	return mapped, nil
}

func mapOperationResults(results []workspace.RepositoryOutcome) []OperationResult {
	mapped := make([]OperationResult, len(results))
	for index, result := range results {
		mapped[index] = OperationResult{
			Repository: mapRepository(result.Repository),
			Outcome:    mapOutcome(result.Outcome),
			Message:    result.Message,
			Diagnostic: result.Diagnostic,
		}
		if result.Diagnostic != nil {
			mapped[index].Error = translateError("Git workspace operation", result.Diagnostic).Error()
		} else if result.Outcome == workspace.OutcomeFailed {
			mapped[index].Error = "The Git workspace operation could not be completed."
		} else if result.Outcome == workspace.OutcomeCancelled {
			mapped[index].Error = "The Git workspace operation was canceled."
		} else if result.Error != "" {
			mapped[index].Error = "The Git workspace operation could not be completed."
		} else {
			mapped[index].Error = result.Error
		}
	}
	return mapped
}

func mapStatus(status workspace.RepositoryStatus) Status {
	return Status{
		Repository: mapRepository(status.Repository),
		Branch: Branch{
			Name:     status.Branch.Name,
			Detached: status.Branch.Detached,
			Commit:   status.Branch.Commit,
		},
		Changes: ChangeCounts{
			Modified:  status.Changes.Modified,
			Added:     status.Changes.Added,
			Deleted:   status.Changes.Deleted,
			Renamed:   status.Changes.Renamed,
			Untracked: status.Changes.Untracked,
			Lines:     append([]string{}, status.Changes.Lines...),
		},
		Sync: SyncStatus{
			Upstream: status.Sync.Upstream,
			Ahead:    status.Sync.Ahead,
			Behind:   status.Sync.Behind,
			State:    SyncState(status.Sync.State),
		},
		Remote: status.Remote,
	}
}

func mapCommits(commits []workspace.Commit) []Commit {
	mapped := make([]Commit, len(commits))
	for index, commit := range commits {
		mapped[index] = Commit{
			Hash:      commit.Hash,
			Message:   commit.Message,
			Author:    commit.Author,
			Timestamp: commit.Timestamp,
		}
	}
	return mapped
}

func mapRepositories(repositories []workspace.Repository) []Repository {
	mapped := make([]Repository, len(repositories))
	for index, repository := range repositories {
		mapped[index] = mapRepository(repository)
	}
	return mapped
}

func mapRepository(repository workspace.Repository) Repository {
	return Repository{Name: repository.Name, Path: repository.Path}
}

func toWorkspaceRepositories(repositories []Repository) []workspace.Repository {
	mapped := make([]workspace.Repository, len(repositories))
	for index, repository := range repositories {
		mapped[index] = toWorkspaceRepository(repository)
	}
	return mapped
}

func toWorkspaceRepository(repository Repository) workspace.Repository {
	return workspace.Repository{Name: repository.Name, Path: repository.Path}
}

func mapOutcome(outcome workspace.Outcome) OperationOutcome {
	if outcome == workspace.OutcomeCancelled {
		return OutcomeCanceled
	}
	return OperationOutcome(outcome)
}

func translateError(operation string, err error) *AdapterError {
	if err == nil {
		return nil
	}
	category := ErrorFailed
	message := "The Git workspace operation could not be completed."
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		category = ErrorCanceled
		message = "The Git workspace operation was canceled."
	case errors.Is(err, workspace.ErrGitNotFound):
		category = ErrorUnavailable
		message = "Git is not available on this system."
	case errors.Is(err, workspace.ErrInvalidOptions):
		category = ErrorInvalidInput
		message = "The Git workspace request is invalid."
	case errors.Is(err, workspace.ErrUnsupportedRemote):
		category = ErrorUnsupported
		message = "The repository remote is unsupported."
	case errors.Is(err, workspace.ErrScanLimit):
		category = ErrorInvalidInput
		message = "The repository scan exceeded its allowed limit."
	case errors.Is(err, workspace.ErrNoUpstream):
		category = ErrorInvalidInput
		message = "The repository has no upstream branch."
	}
	return &AdapterError{
		Category: category,
		Message:  message,
		cause:    fmt.Errorf("%s: %w", operation, err),
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
