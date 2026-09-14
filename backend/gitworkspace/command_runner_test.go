package gitworkspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	workspace "github.com/MisterKeke/GitWorkspaceFun/workspace"
)

type recordedGitCommand struct {
	executable string
	args       []string
}

func TestBackgroundGitRunnerUsesFixedCommandsAndMapsStatus(t *testing.T) {
	commands := make([]recordedGitCommand, 0)
	runner := &backgroundGitRunner{
		lookPath: func(string) (string, error) { return `C:\Program Files\Git\cmd\git.exe`, nil },
		execute: func(_ context.Context, executable string, args ...string) (string, error) {
			commands = append(commands, recordedGitCommand{executable: executable, args: append([]string{}, args...)})
			switch strings.Join(args, " ") {
			case "--version":
				return "git version 2.49.0", nil
			case `-C D:\repo branch --show-current`:
				return "main", nil
			case `-C D:\repo status --porcelain=v1`:
				return " M changed.go\n?? new.go", nil
			case `-C D:\repo rev-parse --abbrev-ref --symbolic-full-name @{upstream}`:
				return "origin/main", nil
			case `-C D:\repo rev-list --left-right --count HEAD...@{upstream}`:
				return "1 2", nil
			case `-C D:\repo remote get-url origin`:
				return "https://example.com/repo.git", nil
			default:
				return "", errors.New("unexpected command")
			}
		},
	}

	environment, err := runner.Detect(context.Background())
	if err != nil || environment.Version != "git version 2.49.0" {
		t.Fatalf("environment=%+v err=%v", environment, err)
	}
	status, err := workspace.New(workspace.Options{Runner: runner}).Status(
		context.Background(),
		workspace.Repository{Name: "repo", Path: `D:\repo`},
	)
	if err != nil {
		t.Fatal(err)
	}
	if status.Branch.Name != "main" || status.Changes.Modified != 1 || status.Changes.Untracked != 1 {
		t.Fatalf("status=%+v", status)
	}
	if status.Sync.State != workspace.SyncDiverged || status.Sync.Ahead != 1 || status.Sync.Behind != 2 {
		t.Fatalf("sync=%+v", status.Sync)
	}
	if status.Remote != "example.com/repo" {
		t.Fatalf("remote=%q", status.Remote)
	}
	if len(commands) != 6 || commands[0].executable != `C:\Program Files\Git\cmd\git.exe` {
		t.Fatalf("commands=%+v", commands)
	}
}

func TestBackgroundGitRunnerTreatsMissingUpstreamAsNoUpstream(t *testing.T) {
	runner := &backgroundGitRunner{execute: func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "@{upstream}") {
			return "", errors.New("fatal: no upstream configured for branch 'main'")
		}
		return "", errors.New("unexpected command")
	}}

	status, err := runner.Sync(context.Background(), `D:\repo`)
	if err != nil || status.State != workspace.SyncNoUpstream {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestBackgroundGitRunnerParsesMultipleHistoryRecords(t *testing.T) {
	stampOne := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	stampTwo := stampOne.Add(time.Hour)
	output := "aaa\x00first\x00A\x00" + stampOne.Format(time.RFC3339) + "\x1e\n" +
		"bbb\x00second\x00B\x00" + stampTwo.Format(time.RFC3339) + "\x1e"
	runner := &backgroundGitRunner{execute: func(context.Context, string, ...string) (string, error) {
		return output, nil
	}}

	commits, err := runner.History(context.Background(), `D:\repo`, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []workspace.Commit{
		{Hash: "aaa", Message: "first", Author: "A", Timestamp: stampOne},
		{Hash: "bbb", Message: "second", Author: "B", Timestamp: stampTwo},
	}
	if !reflect.DeepEqual(commits, want) {
		t.Fatalf("commits=%+v want=%+v", commits, want)
	}
}

func TestBackgroundGitEnvironmentDisablesInteractiveProcesses(t *testing.T) {
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GCM_INTERACTIVE", "Always")
	environment := backgroundGitEnvironment()
	for key, want := range map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GCM_INTERACTIVE":     "Never",
	} {
		matches := make([]string, 0)
		for _, item := range environment {
			name, value, _ := strings.Cut(item, "=")
			if strings.EqualFold(name, key) {
				matches = append(matches, value)
			}
		}
		if !reflect.DeepEqual(matches, []string{want}) {
			t.Fatalf("%s values=%v want=%q", key, matches, want)
		}
	}
}
