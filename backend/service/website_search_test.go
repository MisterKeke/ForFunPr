package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestWebsiteAddressPolicyRejectsNonPublicDestinations(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/",
		"http://[::1]/",
		"http://10.0.0.1/",
		"http://169.254.10.20/",
		"http://224.0.0.1/",
		"http://localhost/",
		"http://printer.local/",
		"http://intranet/",
		"ftp://example.com/",
		"https://user:password@example.com/",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			parsed, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := validatePublicWebsiteURL(context.Background(), parsed); err == nil {
				t.Fatalf("validatePublicWebsiteURL(%q) unexpectedly succeeded", raw)
			}
		})
	}

	parsed, err := url.Parse("https://93.184.216.34/article")
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePublicWebsiteURL(context.Background(), parsed); err != nil {
		t.Fatalf("public address rejected: %v", err)
	}
}

func TestWebsiteRedirectPolicyRevalidatesEveryDestination(t *testing.T) {
	client := newWebsiteHTTPClient()
	base, _ := http.NewRequest(http.MethodGet, "https://93.184.216.34/start", nil)
	public, _ := http.NewRequest(http.MethodGet, "https://93.184.216.34/next", nil)
	if err := client.client.CheckRedirect(public, []*http.Request{base}); err != nil {
		t.Fatalf("public redirect rejected: %v", err)
	}

	private, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/private", nil)
	err := client.client.CheckRedirect(private, []*http.Request{base})
	var fetchError *websiteFetchError
	if !errors.As(err, &fetchError) || fetchError.Code != "blocked_address" {
		t.Fatalf("private redirect error = %v", err)
	}

	via := make([]*http.Request, websiteMaximumRedirects)
	for index := range via {
		via[index] = base
	}
	err = client.client.CheckRedirect(public, via)
	if !errors.As(err, &fetchError) || fetchError.Code != "redirect_limit" {
		t.Fatalf("redirect limit error = %v", err)
	}
}

func TestResolvePublicWebsiteAddressesRejectsMixedDNSResults(t *testing.T) {
	original := websiteLookupNetIP
	websiteLookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("192.168.1.10"),
		}, nil
	}
	t.Cleanup(func() { websiteLookupNetIP = original })

	_, err := resolvePublicWebsiteAddresses(context.Background(), "example.test")
	var fetchError *websiteFetchError
	if !errors.As(err, &fetchError) || fetchError.Code != "blocked_address" {
		t.Fatalf("mixed DNS result error = %v, want blocked_address", err)
	}
}

func TestNormalizeWebsiteSearchURLCanonicalizesDuplicateKeys(t *testing.T) {
	display, normalized, err := normalizeWebsiteSearchURL(" HTTPS://Example.COM:443/path?q=1#section ")
	if err != nil {
		t.Fatal(err)
	}
	if display != "https://example.com/path?q=1#section" {
		t.Fatalf("display URL = %q", display)
	}
	if normalized != "https://example.com/path?q=1" {
		t.Fatalf("normalized URL = %q", normalized)
	}

	if _, _, err := normalizeWebsiteSearchURL("http://localhost/page"); err == nil {
		t.Fatal("localhost URL unexpectedly accepted")
	}
}

func TestWebsiteSearchTargetCRUDAndQueryValidation(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	target, err := service.AddWebsiteSearchTargetContext(ctx, "https://93.184.216.34/path#first")
	if err != nil {
		t.Fatal(err)
	}
	if target.ID <= 0 || target.URL != "https://93.184.216.34/path#first" {
		t.Fatalf("saved target = %#v", target)
	}
	if _, err := service.AddWebsiteSearchTargetContext(ctx, "https://93.184.216.34/path#second"); err == nil {
		t.Fatal("fragment-only duplicate target unexpectedly accepted")
	}
	state, err := service.GetWebsiteSearchStateContext(ctx)
	if err != nil || len(state.Targets) != 1 || state.Targets[0].ID != target.ID {
		t.Fatalf("website state = %#v, %v", state, err)
	}
	if err := service.DeleteWebsiteSearchTargetContext(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteWebsiteSearchTargetContext(ctx, target.ID); err == nil {
		t.Fatal("missing target deletion unexpectedly succeeded")
	}
	if _, err := service.SearchWebsitesContext(ctx, WebsiteSearchRequest{Query: "needle"}); err == nil {
		t.Fatal("website search without targets unexpectedly succeeded")
	}
	for _, query := range []string{" ", strings.Repeat("x", maximumWebsiteSearchQueryRunes+1)} {
		if _, err := normalizeWebsiteSearchQuery(query); err == nil {
			t.Fatalf("invalid query accepted: %q", query)
		}
	}
}

func TestFetchWebsiteHTTPBoundsContentAndExtractsVisibleText(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		wantTitle   string
		wantBlock   string
		wantCode    string
	}{
		{
			name:        "html",
			contentType: "text/html; charset=utf-8",
			body: `<html><head><title> Example title </title><style>.hidden{}</style></head>
				<body><nav>Noise</nav><main><h1>Heading</h1><p>Visible   phrase</p><script>secret phrase</script></main></body></html>`,
			wantTitle: "Example title",
			wantBlock: "Visible phrase",
		},
		{
			name:        "plain text",
			contentType: "text/plain",
			body:        "first line\nsecond phrase",
			wantBlock:   "second phrase",
		},
		{
			name:        "unsupported",
			contentType: "application/pdf",
			body:        "%PDF",
			wantCode:    "unsupported_content",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService()
			service.websiteHTTPClient = &websiteHTTPClient{client: &http.Client{
				Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     http.Header{"Content-Type": []string{testCase.contentType}},
						Body:       io.NopCloser(strings.NewReader(testCase.body)),
						Request:    request,
					}, nil
				}),
			}}

			document, err := service.fetchWebsiteHTTP(context.Background(), "http://93.184.216.34/page")
			if testCase.wantCode != "" {
				var fetchError *websiteFetchError
				if !errors.As(err, &fetchError) || fetchError.Code != testCase.wantCode {
					t.Fatalf("error = %v, want code %q", err, testCase.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if document.Title != testCase.wantTitle {
				t.Fatalf("title = %q, want %q", document.Title, testCase.wantTitle)
			}
			if !strings.Contains(strings.Join(document.TextBlocks, "|"), testCase.wantBlock) {
				t.Fatalf("text blocks = %#v, want %q", document.TextBlocks, testCase.wantBlock)
			}
			if strings.Contains(strings.Join(document.TextBlocks, "|"), "secret phrase") {
				t.Fatalf("script text leaked into visible blocks: %#v", document.TextBlocks)
			}
		})
	}
}

func TestFetchWebsiteHTTPRejectsDeclaredAndStreamedOversizeBodies(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		contentLength int64
		body          string
		wantCode      string
	}{
		{"declared", websiteMaximumResponseBytes + 1, "small", "response_too_large"},
		{"streamed", -1, strings.Repeat("x", websiteMaximumResponseBytes+1), "read_failed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService()
			service.websiteHTTPClient = &websiteHTTPClient{client: &http.Client{
				Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode:    http.StatusOK,
						Status:        "200 OK",
						ContentLength: testCase.contentLength,
						Header:        http.Header{"Content-Type": []string{"text/plain"}},
						Body:          io.NopCloser(strings.NewReader(testCase.body)),
						Request:       request,
					}, nil
				}),
			}}
			_, err := service.fetchWebsiteHTTP(context.Background(), "http://93.184.216.34/page")
			var fetchError *websiteFetchError
			if !errors.As(err, &fetchError) || fetchError.Code != testCase.wantCode {
				t.Fatalf("oversized body error = %v", err)
			}
		})
	}
}

func TestWebsiteMatchingAndTextFragmentAreUnicodeSafe(t *testing.T) {
	text := "Başlangıç — ISTANBUL has one result. Later istanbul has another result."
	count, snippets := findWebsiteMatches(text, "istanbul")
	if count != 2 || len(snippets) == 0 {
		t.Fatalf("matches = %d, snippets = %#v", count, snippets)
	}
	fragment := websiteTextFragmentURL("https://example.com/page#old", "two words")
	if fragment != "https://example.com/page#:~:text=two%20words" {
		t.Fatalf("text fragment = %q", fragment)
	}
}

func TestWebsiteBrowserFallbackPolicyAndGuardState(t *testing.T) {
	for _, code := range []string{"blocked_address", "invalid_url", "dns_failed", "timeout", "canceled", "response_too_large"} {
		if shouldTryWebsiteBrowserFallback(WebsiteSearchPageResult{ErrorCode: code}) {
			t.Fatalf("fallback allowed for terminal error %q", code)
		}
	}
	if shouldTryWebsiteBrowserFallback(WebsiteSearchPageResult{MatchCount: 1}) {
		t.Fatal("fallback allowed for a successful match")
	}
	if !shouldTryWebsiteBrowserFallback(WebsiteSearchPageResult{ErrorCode: "unsupported_content"}) {
		t.Fatal("fallback not allowed for browser-renderable content")
	}

	guard := newWebsiteBrowserRequestGuard()
	guard.recordBlocked("first", false)
	guard.recordBlocked("second", true)
	if guard.blockedMessage() != "first" || guard.mainDocumentBlockedMessage() != "second" {
		t.Fatalf("guard state = (%q, %q)", guard.blockedMessage(), guard.mainDocumentBlockedMessage())
	}
	if !guard.recordDocumentFrame("main") || guard.recordDocumentFrame("child") {
		t.Fatal("document frame classification is incorrect")
	}
	guard.recordMainResponse("child", 500, "ignored")
	guard.recordMainResponse("main", 204, "https://example.com/final")
	status, finalURL := guard.mainResponse()
	if status != 204 || finalURL != "https://example.com/final" {
		t.Fatalf("main response = (%d, %q)", status, finalURL)
	}
}

func TestWebsiteSearchRunRoundTripAndHistoryClear(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	response := WebsiteSearchResponse{
		Query: "needle", UseBrowserFallback: true, Checked: 1, Found: 1,
		Results: []WebsiteSearchPageResult{{
			OriginalURL: "https://example.com/", FinalURL: "https://example.com/final",
			Title: "Example", Hostname: "example.com", MatchCount: 2,
			Snippets: []string{"a needle here"}, FetchMethod: "http",
			CheckedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339),
		}},
	}
	if err := service.storeWebsiteSearchRunContext(ctx, &response); err != nil {
		t.Fatal(err)
	}
	loaded, err := service.GetWebsiteSearchRunContext(ctx, response.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Query != response.Query || loaded.Found != 1 || len(loaded.Results) != 1 ||
		len(loaded.Results[0].Snippets) != 1 {
		t.Fatalf("loaded response = %#v", loaded)
	}
	if err := service.ClearWebsiteSearchHistoryContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetWebsiteSearchRunContext(ctx, response.RunID); err == nil {
		t.Fatal("cleared website search run still exists")
	}
}

func TestCompactWebsiteErrorTruncatesUntrustedMessages(t *testing.T) {
	message := compactWebsiteError(errors.New(strings.Repeat("x", 300)))
	if len(message) != 243 || !strings.HasSuffix(message, "…") {
		t.Fatalf("compacted message length/suffix = %d/%q", len(message), message)
	}
}
