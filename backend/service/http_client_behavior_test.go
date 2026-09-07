package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadLimitedResponseBodyEnforcesExactBoundary(t *testing.T) {
	data, err := readLimitedResponseBody(strings.NewReader("12345"), 5)
	if err != nil || string(data) != "12345" {
		t.Fatalf("exact boundary = %q, %v", data, err)
	}
	if _, err := readLimitedResponseBody(strings.NewReader("123456"), 5); err == nil {
		t.Fatal("oversized response unexpectedly succeeded")
	}
	failing := &failingReader{err: errors.New("read failed")}
	if _, err := readLimitedResponseBody(failing, 5); !errors.Is(err, failing.err) {
		t.Fatalf("reader error = %v", err)
	}
}

type failingReader struct{ err error }

func (reader *failingReader) Read([]byte) (int, error) { return 0, reader.err }

func TestExternalHTTPClientRequestPolicy(t *testing.T) {
	var received *http.Request
	client := &externalHTTPClient{
		maxResponseBytes: 8,
		client: &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
			received = request
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader("response")), Request: request,
			}, nil
		})},
	}
	requestURL, _ := url.Parse("https://example.com/path")
	body, status, err := client.get(context.Background(), "Provider", requestURL,
		http.Header{"X-Test": []string{"one", "two"}}, http.StatusOK)
	if err != nil || status != http.StatusOK || string(body) != "response" {
		t.Fatalf("response = %q, %d, %v", body, status, err)
	}
	if received == nil || len(received.Header.Values("X-Test")) != 2 {
		t.Fatalf("request headers = %#v", received)
	}

	for _, raw := range []string{"http://example.com", "https:///missing-host"} {
		parsed, _ := url.Parse(raw)
		if _, _, err := client.get(context.Background(), "Provider", parsed, nil, http.StatusOK); err == nil {
			t.Fatalf("invalid provider URL accepted: %q", raw)
		}
	}
}

func TestExternalHTTPClientRejectsStatusAndOversizedBodies(t *testing.T) {
	requestURL, _ := url.Parse("https://example.com/")
	cases := []struct {
		name          string
		status        int
		contentLength int64
		body          string
	}{
		{"status", http.StatusTeapot, -1, "no"},
		{"declared length", http.StatusOK, 6, "123456"},
		{"streamed length", http.StatusOK, -1, "123456"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &externalHTTPClient{
				maxResponseBytes: 5,
				client: &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: testCase.status, Status: http.StatusText(testCase.status),
						ContentLength: testCase.contentLength, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader(testCase.body)), Request: request,
					}, nil
				})},
			}
			if _, _, err := client.get(context.Background(), "Provider", requestURL, nil, http.StatusOK); err == nil {
				t.Fatal("bounded request unexpectedly succeeded")
			}
		})
	}
}

func TestExternalHTTPClientMapsCancellation(t *testing.T) {
	requestURL, _ := url.Parse("https://example.com/")
	client := &externalHTTPClient{
		maxResponseBytes: 10,
		client: &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := client.get(ctx, "Provider", requestURL, nil, http.StatusOK)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestExternalHTTPRedirectPolicy(t *testing.T) {
	client := newExternalHTTPClient()
	base, _ := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	allowed, _ := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
	if err := client.client.CheckRedirect(allowed, []*http.Request{base}); err != nil {
		t.Fatalf("same-host HTTPS redirect rejected: %v", err)
	}
	for _, raw := range []string{"http://example.com/downgrade", "https://other.example/next"} {
		request, _ := http.NewRequest(http.MethodGet, raw, nil)
		if err := client.client.CheckRedirect(request, []*http.Request{base}); !errors.Is(err, http.ErrUseLastResponse) {
			t.Fatalf("redirect %q error = %v", raw, err)
		}
	}
	if err := client.client.CheckRedirect(allowed, []*http.Request{base, base, base}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect limit error = %v", err)
	}
}

func TestRunBoundedVisitsEveryItemWithinLimit(t *testing.T) {
	const count = 40
	const limit = 3
	var active atomic.Int32
	var maximum atomic.Int32
	visits := make([]atomic.Int32, count)
	runBounded(context.Background(), count, limit, func(_ context.Context, index int) {
		current := active.Add(1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		visits[index].Add(1)
		active.Add(-1)
	})
	if maximum.Load() > limit {
		t.Fatalf("maximum concurrency = %d, limit %d", maximum.Load(), limit)
	}
	for index := range visits {
		if visits[index].Load() != 1 {
			t.Fatalf("item %d visits = %d", index, visits[index].Load())
		}
	}
}
