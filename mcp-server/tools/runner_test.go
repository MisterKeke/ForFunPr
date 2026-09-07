package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRunExecutesCLIAndDecodesCompleteJSON(t *testing.T) {
	var requestMethod string
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestMethod = request.Method
		requestPath = request.URL.Path
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"status":"ok","ready":true}`)
	}))
	defer server.Close()

	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), server.URL+"/")
	output, err := Run[map[string]any](context.Background(), runner, []string{"health"})
	if err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if requestMethod != http.MethodGet || requestPath != "/api/v1/health" {
		t.Fatalf("request = %s %s", requestMethod, requestPath)
	}
	if output["status"] != "ok" || output["ready"] != true {
		t.Fatalf("output = %#v", output)
	}
}

func TestRunRejectsMissingRunnerConfiguration(t *testing.T) {
	if _, err := Run[map[string]any](context.Background(), nil, []string{"health"}); err == nil {
		t.Fatal("Run accepted a nil runner")
	}

	_, err := Run[map[string]any](context.Background(), NewRunner(nil, "  "), []string{"health"})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Kind != ErrorUnavailable {
		t.Fatalf("unconfigured error = %T %v", err, err)
	}
}

type rejectingOutput struct{}

func (rejectingOutput) UnmarshalJSON([]byte) error {
	return errors.New("reject output")
}

func TestRunClassifiesOutputDecodeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"status":"ok"}`)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := Run[rejectingOutput](context.Background(), NewRunner(logger, server.URL), []string{"health"})
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Kind != ErrorInvalidJSON {
		t.Fatalf("decode error = %T %v", err, err)
	}
}

func TestCommandErrorClassification(t *testing.T) {
	runner := NewRunner(nil, "http://127.0.0.1")

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runner.commandError(canceled, errors.New("ignored"), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}

	network := &url.Error{Op: "Get", URL: "http://127.0.0.1", Err: errors.New("refused")}
	err := runner.commandError(context.Background(), network, "")
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Kind != ErrorUnavailable || !errors.Is(err, network) {
		t.Fatalf("network error = %T %v", err, err)
	}

	err = runner.commandError(context.Background(), errors.New(" command failed "), "stderr")
	if !errors.As(err, &runErr) || runErr.Kind != ErrorCommand || runErr.Message != "command failed" {
		t.Fatalf("command error = %T %v", err, err)
	}

	err = runner.commandError(context.Background(), emptyError{}, " fallback stderr ")
	if !errors.As(err, &runErr) || runErr.Message != "fallback stderr" {
		t.Fatalf("stderr fallback = %T %v", err, err)
	}
}

type emptyError struct{}

func (emptyError) Error() string { return "" }

func TestRunErrorFormattingAndUnwrap(t *testing.T) {
	cause := fmt.Errorf("cause")
	err := &RunError{Kind: ErrorCommand, Message: "safe", cause: cause}
	if err.Error() != "safe" || !errors.Is(err, cause) || strings.Contains(err.Error(), "cause") {
		t.Fatalf("RunError = %q, unwrap=%v", err.Error(), errors.Unwrap(err))
	}
}
