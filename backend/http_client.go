package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	externalConnectTimeout       = 5 * time.Second
	externalTLSHandshakeTimeout  = 5 * time.Second
	externalResponseHeaderTimeout = 10 * time.Second
	externalRequestTimeout       = 15 * time.Second
	externalClientTimeout        = 20 * time.Second
	externalMaxResponseBytes     = 2 << 20

	favoriteRefreshWorkerLimit = 4

	providerFrankfurter = "Frankfurter"
	providerTelegram    = "Telegram"
	providerYouTube     = "YouTube"
)

var (
	currencyCodePattern       = regexp.MustCompile(`^[A-Z]{3}$`)
	youTubeChannelIDPattern   = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	youTubeVideoIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	telegramPostIDPattern     = regexp.MustCompile(`^[0-9]+$`)
)

// externalHTTPClient provides the single request policy for every remote
// provider: bounded timing, response size, redirects, and error messages.
type externalHTTPClient struct {
	client           *http.Client
	maxResponseBytes int64
}

func newExternalHTTPClient() *externalHTTPClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   externalConnectTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = externalTLSHandshakeTimeout
	transport.ResponseHeaderTimeout = externalResponseHeaderTimeout
	transport.ExpectContinueTimeout = time.Second
	transport.IdleConnTimeout = 90 * time.Second
	transport.MaxIdleConns = 10
	transport.MaxIdleConnsPerHost = favoriteRefreshWorkerLimit
	transport.MaxConnsPerHost = favoriteRefreshWorkerLimit

	return &externalHTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   externalClientTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 || req.URL.Scheme != "https" ||
					(len(via) > 0 && req.URL.Hostname() != via[0].URL.Hostname()) {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		maxResponseBytes: externalMaxResponseBytes,
	}
}

// get executes an HTTP GET request and returns only a fully bounded response
// body. Callers cannot accidentally parse an unlimited response stream.
func (c *externalHTTPClient) get(
	ctx context.Context,
	provider string,
	requestURL *url.URL,
	headers http.Header,
	allowedStatuses ...int,
) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, externalRequestTimeout)
	defer cancel()

	if requestURL == nil || requestURL.Scheme != "https" || requestURL.Host == "" {
		return nil, 0, fmt.Errorf("%s request setup failed: invalid HTTPS URL", provider)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("%s request setup failed: %w", provider, err)
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return nil, 0, fmt.Errorf("%s request timed out", provider)
		case errors.Is(ctx.Err(), context.Canceled):
			return nil, 0, fmt.Errorf("%s request canceled", provider)
		default:
			return nil, 0, fmt.Errorf("%s request failed: %w", provider, err)
		}
	}
	defer resp.Body.Close()

	if !hasAllowedHTTPStatus(resp.StatusCode, allowedStatuses) {
		return nil, resp.StatusCode, fmt.Errorf("%s returned HTTP %s", provider, resp.Status)
	}

	if resp.ContentLength > c.maxResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf(
			"%s response exceeds the %d-byte limit",
			provider,
			c.maxResponseBytes,
		)
	}

	body, err := readLimitedResponseBody(resp.Body, c.maxResponseBytes)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%s response could not be read: %w", provider, err)
	}

	return body, resp.StatusCode, nil
}

func hasAllowedHTTPStatus(status int, allowed []int) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func readLimitedResponseBody(body io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response exceeds the %d-byte limit", maxBytes)
	}
	return data, nil
}

func (c *externalHTTPClient) closeIdleConnections() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}

func (a *App) requestContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func runBounded(ctx context.Context, count int, limit int, work func(context.Context, int)) {
	if count <= 0 {
		return
	}
	if limit < 1 {
		limit = 1
	}
	if limit > count {
		limit = count
	}

	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(limit)
	for range limit {
		go func() {
			defer workers.Done()
			for index := range jobs {
				work(ctx, index)
			}
		}()
	}

	for index := range count {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
}

func normalizeCurrency(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !currencyCodePattern.MatchString(code) {
		return ""
	}
	return code
}

func normalizeTelegramUsername(username string) string {
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	username = strings.ToLower(username)
	if len(username) < 5 || len(username) > 32 {
		return ""
	}
	for _, character := range username {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '_' {
			return ""
		}
	}
	return username
}

func normalizeYouTubeChannelID(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if !youTubeChannelIDPattern.MatchString(channelID) {
		return ""
	}
	return channelID
}

func normalizeYouTubeUsername(username string) string {
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	if username == "" || !utf8.ValidString(username) {
		return ""
	}

	length := utf8.RuneCountInString(username)
	if length < 3 || length > 30 {
		return ""
	}
	for _, character := range username {
		if !unicode.IsLetter(character) && !unicode.IsNumber(character) &&
			character != '_' && character != '-' && character != '.' {
			return ""
		}
	}
	return username
}

func normalizeYouTubeVideoID(videoID string) string {
	videoID = strings.TrimSpace(videoID)
	if !youTubeVideoIDPattern.MatchString(videoID) {
		return ""
	}
	return videoID
}
