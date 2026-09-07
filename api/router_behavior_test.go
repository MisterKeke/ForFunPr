package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	backend "something/backend/service"
)

func newReadyAPITestService(t *testing.T) *backend.Service {
	t.Helper()
	dataRoot := t.TempDir()
	t.Setenv("APPDATA", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", dataRoot)
	service := backend.NewService()
	service.Startup(context.Background())
	if status := service.GetStartupStatus(); !status.Ready {
		t.Fatalf("test service did not start: %#v", status)
	}
	t.Cleanup(func() { service.Shutdown(context.Background()) })
	return service
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload errorBody
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body %q: %v", response.Body.String(), err)
	}
	return payload.Error.Code
}

func TestDecodeJSONBodyContract(t *testing.T) {
	type input struct {
		Value string `json:"value"`
	}
	cases := []struct {
		name        string
		contentType string
		body        string
		wantOK      bool
		wantStatus  int
		wantCode    string
	}{
		{"valid", "application/json", `{"value":"ok"}`, true, http.StatusOK, ""},
		{"vendor JSON", "application/problem+json; charset=utf-8", `{"value":"ok"}`, true, http.StatusOK, ""},
		{"missing type", "", `{"value":"ok"}`, false, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"empty", "application/json", ``, false, http.StatusBadRequest, "missing_json_body"},
		{"malformed", "application/json", `{`, false, http.StatusBadRequest, "invalid_json"},
		{"unknown field", "application/json", `{"extra":true}`, false, http.StatusBadRequest, "invalid_json"},
		{"trailing object", "application/json", `{"value":"ok"}{}`, false, http.StatusBadRequest, "invalid_json"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(testCase.body))
			if testCase.contentType != "" {
				request.Header.Set("Content-Type", testCase.contentType)
			}
			response := httptest.NewRecorder()
			var destination input
			ok := decodeJSONBody(response, request, &destination)
			if ok != testCase.wantOK {
				t.Fatalf("decode result = %v", ok)
			}
			if ok {
				if destination.Value != "ok" {
					t.Fatalf("decoded value = %q", destination.Value)
				}
				return
			}
			if response.Code != testCase.wantStatus || errorCode(t, response) != testCase.wantCode {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}

	largeBody := `{"value":"` + strings.Repeat("x", int(maximumJSONBodyBytes)) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(largeBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	if decodeJSONBody(response, request, &input{}) || response.Code != http.StatusRequestEntityTooLarge ||
		errorCode(t, response) != "request_too_large" {
		t.Fatalf("oversized response = %d %s", response.Code, response.Body.String())
	}
}

func TestResponseWritersSetContractHeadersAndHandleEncodingFailure(t *testing.T) {
	response := httptest.NewRecorder()
	writeJSON(response, http.StatusCreated, map[string]string{"status": "ok"})
	if response.Code != http.StatusCreated ||
		response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.HasSuffix(response.Body.String(), "\n") {
		t.Fatalf("JSON response = %d %#v %q", response.Code, response.Header(), response.Body.String())
	}

	response = httptest.NewRecorder()
	writeJSON(response, http.StatusOK, make(chan int))
	if response.Code != http.StatusInternalServerError || errorCode(t, response) != "encoding_error" {
		t.Fatalf("encoding failure = %d %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	writeNoContent(response)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("no-content response = %d %#v %q", response.Code, response.Header(), response.Body.String())
	}
}

func TestRouterHealthAndOperationReadiness(t *testing.T) {
	notReady := newRouter(backend.NewService())
	response := httptest.NewRecorder()
	notReady.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-ready health status = %d", response.Code)
	}
	response = httptest.NewRecorder()
	notReady.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
	if response.Code != http.StatusServiceUnavailable || errorCode(t, response) != "backend_not_ready" {
		t.Fatalf("not-ready operation = %d %s", response.Code, response.Body.String())
	}

	ready := newRouter(newReadyAPITestService(t))
	response = httptest.NewRecorder()
	ready.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusOK || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("ready health = %d %#v", response.Code, response.Header())
	}
}

func TestTaskRoutesValidateAndMutate(t *testing.T) {
	router := newRouter(newReadyAPITestService(t))
	response := performAPIRequest(router, http.MethodPost, "/api/v1/tasks",
		`{"title":" Ship release ","priority":"HIGH","due_date":"2028-02-29","difficulty":"hard","tags":["Go","go"],"subtasks":[{"title":"Package"}]}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create task = %d %s", response.Code, response.Body.String())
	}
	var created []taskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0].Title != "Ship release" || created[0].Priority != "high" || len(created[0].Tags) != 1 {
		t.Fatalf("created tasks = %#v", created)
	}
	id := created[0].ID

	response = performAPIRequest(router, http.MethodPost, "/api/v1/tasks", `{"title":"bad","priority":"urgent"}`)
	if response.Code != http.StatusUnprocessableEntity || errorCode(t, response) != "invalid_task_priority" {
		t.Fatalf("invalid priority = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodPut, "/api/v1/tasks/not-an-id", `{"title":"x","priority":"low"}`)
	if response.Code != http.StatusBadRequest || errorCode(t, response) != "invalid_task_id" {
		t.Fatalf("invalid task ID = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodPost, "/api/v1/tasks/"+itoa(id)+"/toggle", `{}`)
	if response.Code != http.StatusOK {
		t.Fatalf("toggle task = %d %s", response.Code, response.Body.String())
	}
	var toggled []taskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &toggled); err != nil || len(toggled) != 1 || !toggled[0].Done {
		t.Fatalf("toggled tasks = %#v, %v", toggled, err)
	}
	response = performAPIRequest(router, http.MethodDelete, "/api/v1/tasks/"+itoa(id), "")
	if response.Code != http.StatusOK {
		t.Fatalf("delete task = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodDelete, "/api/v1/tasks/"+itoa(id), "")
	if response.Code != http.StatusNotFound || errorCode(t, response) != "task_not_found" {
		t.Fatalf("delete missing task = %d %s", response.Code, response.Body.String())
	}
}

func TestOrganizerRoutesMapConflictsAndNotFound(t *testing.T) {
	router := newRouter(newReadyAPITestService(t))
	response := performAPIRequest(router, http.MethodPost, "/api/v1/notes", `{"title":"One","body":"body"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create note = %d %s", response.Code, response.Body.String())
	}
	var note backend.Note
	if err := json.Unmarshal(response.Body.Bytes(), &note); err != nil {
		t.Fatal(err)
	}
	response = performAPIRequest(router, http.MethodPut, "/api/v1/notes/"+itoa(note.ID)+"/pinned",
		`{"value":true,"expected_revision":1}`)
	if response.Code != http.StatusOK {
		t.Fatalf("pin note = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodPut, "/api/v1/notes/"+itoa(note.ID),
		`{"title":"stale","body":"body","expected_revision":1}`)
	if response.Code != http.StatusConflict || errorCode(t, response) != "note_conflict" {
		t.Fatalf("stale note = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodGet, "/api/v1/notes/999999", "")
	if response.Code != http.StatusNotFound || errorCode(t, response) != "note_not_found" {
		t.Fatalf("missing note = %d %s", response.Code, response.Body.String())
	}

	bookmarkBody := `{"url":"https://example.com/path","title":"Example","tags":["one"]}`
	response = performAPIRequest(router, http.MethodPost, "/api/v1/bookmarks", bookmarkBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("create bookmark = %d %s", response.Code, response.Body.String())
	}
	response = performAPIRequest(router, http.MethodPost, "/api/v1/bookmarks", bookmarkBody)
	if response.Code != http.StatusConflict || errorCode(t, response) != "bookmark_conflict" {
		t.Fatalf("duplicate bookmark = %d %s", response.Code, response.Body.String())
	}
}

func performAPIRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" || method == http.MethodPost || method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func TestQueryAndPathParsingHelpers(t *testing.T) {
	for _, raw := range []string{"0", "42"} {
		request := httptest.NewRequest(http.MethodGet, "/?before="+url.QueryEscape(raw), nil)
		cursor, present, err := paginationCursor(request)
		if err != nil || !present || (raw == "42" && cursor != 42) {
			t.Fatalf("cursor %q = %d, %v, %v", raw, cursor, present, err)
		}
	}
	for _, raw := range []string{"", "-1", "word"} {
		request := httptest.NewRequest(http.MethodGet, "/?before="+raw, nil)
		if _, present, err := paginationCursor(request); !present || err == nil {
			t.Fatalf("invalid cursor %q accepted", raw)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, present, err := paginationCursor(request); present || err != nil {
		t.Fatalf("absent cursor = %v, %v", present, err)
	}

	symbols, err := parseCurrencySymbols(" usd, EUR,usd ")
	if err != nil || len(symbols) != 2 || symbols[0] != "USD" || symbols[1] != "EUR" {
		t.Fatalf("currency symbols = %#v, %v", symbols, err)
	}
	if _, err := parseCurrencySymbols("USD,invalid"); err == nil {
		t.Fatal("invalid currency symbols accepted")
	}
}

func TestExecutionConfirmationRequiresExplicitTrue(t *testing.T) {
	for _, body := range []string{`{}`, `{"confirm":false}`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		if decodeExecutionConfirmation(response, request) || response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("confirmation %s = %d %s", body, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"confirm":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	if !decodeExecutionConfirmation(response, request) {
		t.Fatalf("true confirmation rejected: %d %s", response.Code, response.Body.String())
	}
}
