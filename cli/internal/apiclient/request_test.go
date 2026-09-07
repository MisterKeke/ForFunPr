package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClientValidatesBaseURL(t *testing.T) {
	for _, raw := range []string{
		"", "localhost:8080", "ftp://localhost", "http://user:pass@localhost",
		"http://localhost?query=1", "http://localhost#fragment",
	} {
		if _, err := New(raw); err == nil {
			t.Fatalf("invalid API URL accepted: %q", raw)
		}
	}
	client, err := New(" http://127.0.0.1:8080/base/ ")
	if err != nil || client.baseURL.Path != "/base/" {
		t.Fatalf("valid API URL = %#v, %v", client, err)
	}
}

func TestDoJSONBuildsRequestAndDecodesResponse(t *testing.T) {
	type requestBody struct {
		Name string `json:"name"`
	}
	type responseBody struct {
		OK bool `json:"ok"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/base/items" || request.URL.Query().Get("page") != "2" {
			t.Errorf("request = %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers = %#v", request.Header)
		}
		var body requestBody
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.Name != "example" {
			t.Errorf("request body = %#v, %v", body, err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"ok":true}`)
	}))
	defer server.Close()
	client, err := New(server.URL + "/base/")
	if err != nil {
		t.Fatal(err)
	}
	var destination responseBody
	if err := client.doJSON(context.Background(), http.MethodPost, "/items",
		url.Values{"page": []string{"2"}}, requestBody{Name: "example"}, &destination); err != nil {
		t.Fatal(err)
	}
	if !destination.OK {
		t.Fatalf("decoded response = %#v", destination)
	}
}

func TestDoJSONHandlesNoContentMalformedSuccessAndAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/empty":
			writer.WriteHeader(http.StatusNoContent)
		case "/malformed":
			_, _ = io.WriteString(writer, `{`)
		case "/api-error":
			writer.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(writer, `{"error":{"code":"duplicate","message":"Already exists"}}`)
		case "/plain-error":
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(writer, `not json`)
		}
	}))
	defer server.Close()
	client, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.doJSON(context.Background(), http.MethodDelete, "/empty", nil, nil, nil); err != nil {
		t.Fatalf("204 response: %v", err)
	}
	var destination map[string]any
	if err := client.doJSON(context.Background(), http.MethodGet, "/malformed", nil, nil, &destination); err == nil ||
		!strings.Contains(err.Error(), "decode API response") {
		t.Fatalf("malformed success error = %v", err)
	}
	for path, want := range map[string]struct {
		status  int
		code    string
		message string
	}{
		"/api-error":   {http.StatusConflict, "duplicate", "Already exists"},
		"/plain-error": {http.StatusBadGateway, "", http.StatusText(http.StatusBadGateway)},
	} {
		err := client.doJSON(context.Background(), http.MethodGet, path, nil, nil, nil)
		var apiError *Error
		if !errors.As(err, &apiError) || apiError.StatusCode != want.status ||
			apiError.Code != want.code || apiError.Message != want.message {
			t.Fatalf("%s error = %#v", path, err)
		}
	}
}

func TestDoJSONPropagatesContextCancellation(t *testing.T) {
	client, err := New("http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = apiRoundTripper(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = client.doJSON(ctx, http.MethodGet, "/", nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

type apiRoundTripper func(*http.Request) (*http.Response, error)

func (fn apiRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
