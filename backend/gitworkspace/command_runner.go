package gitworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	workspace "github.com/MisterKeke/GitWorkspaceFun/workspace"
)

const (
	gitDetectionTimeout = 5 * time.Second
	gitWaitDelay        = 2 * time.Second
	gitCommitFormat     = "%H%x00%s%x00%an%x00%aI%x1e"
)

type gitCommandExecutor func(context.Context, string, ...string) (string, error)

type backgroundGitRunner struct {
	lookPath func(string) (string, error)
	execute  gitCommandExecutor
}

var _ workspace.Runner = (*backgroundGitRunner)(nil)

func newBackgroundGitRunner() workspace.Runner {
	return &backgroundGitRunner{
		lookPath: exec.LookPath,
		execute:  executeBackgroundCommand,
	}
}

func (r *backgroundGitRunner) Detect(ctx context.Context) (workspace.Environment, error) {
	if ctx == nil {
		return workspace.Environment{}, workspace.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return workspace.Environment{}, err
	}
	path, err := r.lookPath("git")
	if err != nil {
		return workspace.Environment{}, workspace.ErrGitNotFound
	}
	opCtx, cancel := context.WithTimeout(ctx, gitDetectionTimeout)
	defer cancel()
	output, err := r.execute(opCtx, path, "--version")
	if err != nil {
		return workspace.Environment{}, fmt.Errorf("detect Git: %w", err)
	}
	return workspace.Environment{GitPath: path, Version: strings.TrimSpace(output)}, nil
}

func (r *backgroundGitRunner) Branch(ctx context.Context, repositoryPath string) (workspace.Branch, error) {
	branch, err := r.runGit(ctx, repositoryPath, "branch", "--show-current")
	if err != nil {
		return workspace.Branch{}, err
	}
	if branch != "" {
		return workspace.Branch{Name: branch}, nil
	}
	commit, err := r.runGit(ctx, repositoryPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return workspace.Branch{}, fmt.Errorf("resolve detached HEAD: %w", err)
	}
	return workspace.Branch{Name: "detached@" + commit, Detached: true, Commit: commit}, nil
}

func (r *backgroundGitRunner) Status(ctx context.Context, repositoryPath string) (workspace.Changes, error) {
	output, err := r.runGit(ctx, repositoryPath, "status", "--porcelain=v1")
	if err != nil {
		return workspace.Changes{}, err
	}
	return workspace.ParseChanges(output)
}

func (r *backgroundGitRunner) Sync(ctx context.Context, repositoryPath string) (workspace.SyncStatus, error) {
	upstream, err := r.runGit(ctx, repositoryPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "upstream") {
			return workspace.SyncStatus{State: workspace.SyncNoUpstream}, nil
		}
		return workspace.SyncStatus{}, err
	}
	if upstream == "" {
		return workspace.SyncStatus{State: workspace.SyncNoUpstream}, nil
	}
	output, err := r.runGit(ctx, repositoryPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return workspace.SyncStatus{}, err
	}
	ahead, behind, err := workspace.ParseSyncCounts(output)
	if err != nil {
		return workspace.SyncStatus{}, err
	}
	state := workspace.SyncClean
	switch {
	case ahead > 0 && behind > 0:
		state = workspace.SyncDiverged
	case ahead > 0:
		state = workspace.SyncAhead
	case behind > 0:
		state = workspace.SyncBehind
	}
	return workspace.SyncStatus{Upstream: upstream, Ahead: ahead, Behind: behind, State: state}, nil
}

func (r *backgroundGitRunner) RemoteURL(ctx context.Context, repositoryPath, remote string) (string, error) {
	return r.runGit(ctx, repositoryPath, "remote", "get-url", remote)
}

func (r *backgroundGitRunner) LatestCommit(ctx context.Context, repositoryPath string) (workspace.Commit, error) {
	output, err := r.runGit(ctx, repositoryPath, "log", "-1", "--format="+gitCommitFormat)
	if err != nil {
		return workspace.Commit{}, err
	}
	return parseGitCommit(output)
}

func (r *backgroundGitRunner) History(ctx context.Context, repositoryPath string, limit int) ([]workspace.Commit, error) {
	if limit <= 0 {
		return []workspace.Commit{}, workspace.ErrInvalidOptions
	}
	output, err := r.runGit(ctx, repositoryPath, "log", "-n", strconv.Itoa(limit), "--format="+gitCommitFormat)
	if err != nil {
		return []workspace.Commit{}, err
	}
	return parseGitCommits(output)
}

func (r *backgroundGitRunner) Fetch(ctx context.Context, repositoryPath string) error {
	_, err := r.runGit(ctx, repositoryPath, "fetch")
	return err
}

func (r *backgroundGitRunner) PullFastForward(ctx context.Context, repositoryPath string) error {
	_, err := r.runGit(ctx, repositoryPath, "pull", "--ff-only")
	return err
}

func (r *backgroundGitRunner) runGit(ctx context.Context, repositoryPath string, args ...string) (string, error) {
	gitArgs := make([]string, 0, len(args)+2)
	gitArgs = append(gitArgs, "-C", repositoryPath)
	gitArgs = append(gitArgs, args...)
	return r.execute(ctx, "git", gitArgs...)
}

func executeBackgroundCommand(ctx context.Context, executable string, args ...string) (string, error) {
	if ctx == nil {
		return "", workspace.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	command := exec.CommandContext(ctx, executable, args...)
	configureBackgroundCommand(command)
	command.Env = backgroundGitEnvironment()
	command.WaitDelay = gitWaitDelay
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Start(); err != nil {
		return "", fmt.Errorf("start Git: %w", err)
	}
	release, err := containBackgroundProcess(command.Process)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return "", fmt.Errorf("contain Git process: %w", err)
	}
	defer release()

	err = command.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("Git command failed: %w: %s", err, detail)
		}
		return "", fmt.Errorf("Git command failed: %w", err)
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

func backgroundGitEnvironment() []string {
	environment := os.Environ()
	for key, value := range map[string]string{
		"GCM_INTERACTIVE":     "Never",
		"GIT_PAGER":           "cat",
		"GIT_TERMINAL_PROMPT": "0",
		"PAGER":               "cat",
	} {
		filtered := environment[:0]
		for _, item := range environment {
			name, _, _ := strings.Cut(item, "=")
			if !strings.EqualFold(name, key) {
				filtered = append(filtered, item)
			}
		}
		environment = append(filtered, key+"="+value)
	}
	return environment
}

func parseGitCommits(output string) ([]workspace.Commit, error) {
	output = strings.Trim(output, "\r\n\x1e")
	if output == "" {
		return []workspace.Commit{}, nil
	}
	records := strings.Split(output, "\x1e")
	commits := make([]workspace.Commit, 0, len(records))
	for _, record := range records {
		record = strings.Trim(record, "\r\n")
		if record == "" {
			continue
		}
		commit, err := parseGitCommit(record)
		if err != nil {
			return []workspace.Commit{}, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func parseGitCommit(output string) (workspace.Commit, error) {
	fields := strings.SplitN(strings.Trim(output, "\r\n\x1e"), "\x00", 4)
	if len(fields) != 4 {
		return workspace.Commit{}, errors.New("Git returned an invalid commit record")
	}
	timestamp, err := time.Parse(time.RFC3339, fields[3])
	if err != nil {
		return workspace.Commit{}, fmt.Errorf("parse Git commit timestamp: %w", err)
	}
	return workspace.Commit{Hash: fields[0], Message: fields[1], Author: fields[2], Timestamp: timestamp}, nil
}
