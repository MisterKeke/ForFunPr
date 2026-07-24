package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"currency-wails/cli/something/cmd"
)

const desktopAPIURL = "http://127.0.0.1:8080"

// ErrorKind identifies a safe, actionable CLI runner failure category.
type ErrorKind string

const (
	// ErrorUnavailable means the loopback desktop REST API could not be reached.
	ErrorUnavailable ErrorKind = "desktop_api_unavailable"
	// ErrorInvalidJSON means a successful CLI command emitted invalid JSON.
	ErrorInvalidJSON ErrorKind = "invalid_cli_json"
	// ErrorCommand means the CLI or desktop API rejected the fixed command.
	ErrorCommand ErrorKind = "cli_command_failed"
)

// RunError is a safe MCP-facing error with an underlying error retained for
// local logging and cancellation inspection.
type RunError struct {
	Kind    ErrorKind
	Message string
	cause   error
}

func (err *RunError) Error() string {
	return err.Message
}

func (err *RunError) Unwrap() error {
	return err.cause
}

// Runner executes fixed Something CLI command paths in-process.
type Runner struct {
	logger *slog.Logger
}

// NewRunner creates an in-process CLI runner.
func NewRunner(logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{logger: logger}
}

// Run executes CLI arguments with forced JSON output and desktop API URL,
// then decodes the complete JSON result into Out.
func Run[Out any](
	ctx context.Context,
	runner *Runner,
	args []string,
) (Out, error) {
	var output Out
	if runner == nil {
		return output, errors.New("the Something MCP CLI runner is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	commandArgs := make([]string, 0, len(args)+4)
	commandArgs = append(commandArgs, args...)
	commandArgs = append(
		commandArgs,
		"--output", "json",
		"--api-url", desktopAPIURL,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cmd.ExecuteArgs(ctx, commandArgs, &stdout, &stderr)
	if err != nil {
		runner.logger.Error("Something CLI command failed", "error", err)
		return output, runner.commandError(ctx, err, stderr.String())
	}

	decoder := json.NewDecoder(&stdout)
	if err := decoder.Decode(&output); err != nil {
		runner.logger.Error("Something CLI returned invalid JSON", "error", err)
		return output, &RunError{
			Kind:    ErrorInvalidJSON,
			Message: "The Something CLI returned an invalid JSON response.",
			cause:   err,
		}
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		runner.logger.Error("Something CLI returned trailing output", "error", err)
		return output, &RunError{
			Kind:    ErrorInvalidJSON,
			Message: "The Something CLI returned unexpected trailing output.",
			cause:   err,
		}
	}
	return output, nil
}

func (runner *Runner) commandError(
	ctx context.Context,
	err error,
	stderr string,
) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}

	var urlError *url.Error
	var networkError net.Error
	if errors.As(err, &urlError) || errors.As(err, &networkError) {
		return &RunError{
			Kind: ErrorUnavailable,
			Message: "The Something desktop API is unavailable. " +
				"Make sure the desktop application is running.",
			cause: err,
		}
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = strings.TrimSpace(stderr)
	}
	if message == "" {
		message = "the Something CLI command failed"
	}
	return &RunError{
		Kind:    ErrorCommand,
		Message: message,
		cause:   err,
	}
}
