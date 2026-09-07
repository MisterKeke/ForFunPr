package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	websiteRequestTimeout        = 20 * time.Second
	websiteConnectTimeout        = 10 * time.Second
	websiteResponseHeaderTimeout = 12 * time.Second
	websiteMaximumResponseBytes  = 3 << 20
	websiteMaximumRedirects      = 5
)

// websiteLookupNetIP is a package seam for deterministic DNS-policy tests.
// Production always uses the process' default resolver.
var websiteLookupNetIP = net.DefaultResolver.LookupNetIP

type websiteFetchError struct {
	Code    string
	Message string
}

func (e *websiteFetchError) Error() string {
	if e == nil {
		return "website request failed"
	}
	return e.Message
}

type websiteHTTPClient struct {
	client *http.Client
}

func newWebsiteHTTPClient() *websiteHTTPClient {
	client := &websiteHTTPClient{}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = client.dialPublicContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = websiteResponseHeaderTimeout
	transport.ExpectContinueTimeout = time.Second
	transport.IdleConnTimeout = 60 * time.Second
	transport.MaxIdleConns = websiteSearchWorkerLimit
	transport.MaxIdleConnsPerHost = websiteSearchWorkerLimit
	transport.MaxConnsPerHost = websiteSearchWorkerLimit
	client.client = &http.Client{
		Transport: transport,
		Timeout:   websiteRequestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= websiteMaximumRedirects {
				return &websiteFetchError{Code: "redirect_limit", Message: "The page redirected too many times."}
			}
			if err := validatePublicWebsiteURL(request.Context(), request.URL); err != nil {
				return err
			}
			return nil
		},
	}
	return client
}

func (c *websiteHTTPClient) closeIdleConnections() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}

func (c *websiteHTTPClient) dialPublicContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, &websiteFetchError{Code: "invalid_url", Message: "The page address contains an invalid host or port."}
	}
	addresses, err := resolvePublicWebsiteAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: websiteConnectTimeout, KeepAlive: 30 * time.Second}
	var lastError error
	for _, candidate := range addresses {
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastError = dialErr
	}
	if lastError == nil {
		lastError = errors.New("hostname has no usable addresses")
	}
	return nil, lastError
}

func (a *Service) searchWebsiteTargetHTTP(ctx context.Context, target WebsiteSearchTarget, query string) WebsiteSearchPageResult {
	document, err := a.fetchWebsiteHTTP(ctx, target.URL)
	if err != nil {
		return websiteFailureResult(target, err, "http")
	}
	return resultFromWebsiteDocument(target, query, document)
}

func (a *Service) fetchWebsiteHTTP(ctx context.Context, value string) (websiteDocument, error) {
	if a.websiteHTTPClient == nil || a.websiteHTTPClient.client == nil {
		return websiteDocument{}, &websiteFetchError{Code: "request_failed", Message: "The Website Search HTTP client is unavailable."}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "invalid_url", Message: "The saved page URL is invalid."}
	}
	if err := validatePublicWebsiteURL(ctx, parsed); err != nil {
		return websiteDocument{}, err
	}

	requestContext, cancel := context.WithTimeout(ctx, websiteRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "invalid_url", Message: "The page request could not be created."}
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")
	request.Header.Set("Accept-Language", "en,ru,tr;q=0.8,*;q=0.5")
	request.Header.Set("User-Agent", "Something-WebsiteSearch/1.0")

	response, err := a.websiteHTTPClient.client.Do(request)
	if err != nil {
		var fetchError *websiteFetchError
		switch {
		case errors.As(err, &fetchError):
			return websiteDocument{}, fetchError
		case errors.Is(requestContext.Err(), context.DeadlineExceeded):
			return websiteDocument{}, &websiteFetchError{Code: "timeout", Message: "The page did not respond before the timeout."}
		case errors.Is(requestContext.Err(), context.Canceled):
			return websiteDocument{}, &websiteFetchError{Code: "canceled", Message: "The page request was canceled."}
		default:
			return websiteDocument{}, &websiteFetchError{Code: "request_failed", Message: "The page could not be reached: " + compactWebsiteError(err)}
		}
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return websiteDocument{}, &websiteFetchError{
			Code:    "http_status",
			Message: fmt.Sprintf("The page returned HTTP %s.", response.Status),
		}
	}
	if response.ContentLength > websiteMaximumResponseBytes {
		return websiteDocument{}, &websiteFetchError{Code: "response_too_large", Message: "The page is larger than the 3 MiB search limit."}
	}
	body, err := readLimitedResponseBody(response.Body, websiteMaximumResponseBytes)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "read_failed", Message: "The page content could not be read."}
	}

	contentType := response.Header.Get("Content-Type")
	mediaType, _, mediaTypeErr := mime.ParseMediaType(contentType)
	if mediaTypeErr != nil && contentType != "" {
		return websiteDocument{}, &websiteFetchError{Code: "unsupported_content", Message: "The page returned an invalid content type."}
	}
	if mediaType == "" {
		mediaType = http.DetectContentType(body)
		if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
			mediaType = mediaType[:separator]
		}
	}

	finalURL := response.Request.URL.String()
	if mediaType == "text/plain" {
		textBlocks := splitWebsiteTextBlocks(string(body))
		if len(textBlocks) == 0 {
			return websiteDocument{}, &websiteFetchError{Code: "empty_content", Message: "The page did not contain readable text."}
		}
		return websiteDocument{FinalURL: finalURL, TextBlocks: textBlocks, Method: "http"}, nil
	}
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return websiteDocument{}, &websiteFetchError{Code: "unsupported_content", Message: "The URL did not return an HTML page."}
	}

	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "parse_failed", Message: "The page character encoding could not be read."}
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, websiteMaximumResponseBytes+1))
	if err != nil || len(decoded) > websiteMaximumResponseBytes {
		return websiteDocument{}, &websiteFetchError{Code: "parse_failed", Message: "The decoded page content could not be read."}
	}
	title, textBlocks, err := extractWebsiteHTML(decoded)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "parse_failed", Message: err.Error()}
	}
	if len(textBlocks) == 0 {
		return websiteDocument{}, &websiteFetchError{Code: "empty_content", Message: "The page did not contain readable visible text."}
	}
	return websiteDocument{FinalURL: finalURL, Title: title, TextBlocks: textBlocks, Method: "http"}, nil
}

func validatePublicWebsiteURL(ctx context.Context, parsed *url.URL) error {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return &websiteFetchError{Code: "invalid_url", Message: "Only HTTP and HTTPS page URLs are supported."}
	}
	if parsed.User != nil {
		return &websiteFetchError{Code: "invalid_url", Message: "Page URLs cannot contain credentials."}
	}
	if err := rejectObviousInternalHostname(parsed.Hostname()); err != nil {
		return &websiteFetchError{Code: "blocked_address", Message: err.Error()}
	}
	_, err := resolvePublicWebsiteAddresses(ctx, parsed.Hostname())
	return err
}

func rejectObviousInternalHostname(host string) error {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if normalized == "" {
		return &ValidationError{Field: "url", Message: "Website Search URLs must include a hostname."}
	}
	if address, err := netip.ParseAddr(strings.Trim(normalized, "[]")); err == nil {
		if !isPublicWebsiteAddress(address) {
			return &ValidationError{Field: "url", Message: "Website Search cannot access local or private network addresses."}
		}
		return nil
	}
	blockedSuffixes := []string{"localhost", "local", "internal", "home", "lan"}
	for _, suffix := range blockedSuffixes {
		if normalized == suffix || strings.HasSuffix(normalized, "."+suffix) {
			return &ValidationError{Field: "url", Message: "Website Search cannot access local or private network hostnames."}
		}
	}
	if !strings.Contains(normalized, ".") {
		return &ValidationError{Field: "url", Message: "Website Search cannot access single-label internal hostnames."}
	}
	return nil
}

func resolvePublicWebsiteAddresses(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if address, err := netip.ParseAddr(host); err == nil {
		if !isPublicWebsiteAddress(address) {
			return nil, &websiteFetchError{Code: "blocked_address", Message: "The page resolves to a local or private network address."}
		}
		return []netip.Addr{address.Unmap()}, nil
	}
	addresses, err := websiteLookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, &websiteFetchError{Code: "dns_failed", Message: "The page hostname could not be resolved."}
	}
	if len(addresses) == 0 {
		return nil, &websiteFetchError{Code: "dns_failed", Message: "The page hostname did not resolve to an address."}
	}
	public := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if !isPublicWebsiteAddress(address) {
			return nil, &websiteFetchError{Code: "blocked_address", Message: "The page hostname resolves to a local or private network address."}
		}
		public = append(public, address)
	}
	return public, nil
}

func isPublicWebsiteAddress(address netip.Addr) bool {
	return address.IsValid() && address.IsGlobalUnicast() && !address.IsPrivate() &&
		!address.IsLoopback() && !address.IsUnspecified() &&
		!address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast() &&
		!address.IsMulticast()
}

func websiteFailureResult(target WebsiteSearchTarget, err error, method string) WebsiteSearchPageResult {
	result := WebsiteSearchPageResult{
		TargetID:     target.ID,
		OriginalURL:  target.URL,
		Snippets:     []string{},
		FetchMethod:  method,
		ErrorCode:    "request_failed",
		ErrorMessage: "The page could not be checked.",
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if parsed, parseErr := url.Parse(target.URL); parseErr == nil {
		result.Hostname = parsed.Hostname()
	}
	var fetchError *websiteFetchError
	if errors.As(err, &fetchError) {
		result.ErrorCode = fetchError.Code
		result.ErrorMessage = fetchError.Message
	} else if err != nil {
		result.ErrorMessage = compactWebsiteError(err)
	}
	return result
}

func compactWebsiteError(err error) string {
	if err == nil {
		return "unknown error"
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return message
}
