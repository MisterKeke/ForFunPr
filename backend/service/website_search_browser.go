package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	websiteBrowserPageTimeout  = 25 * time.Second
	websiteBrowserSettleTime   = 1200 * time.Millisecond
	websiteBrowserTextLimit    = 2_000_000
	websiteBrowserRequestLimit = 250
)

func shouldTryWebsiteBrowserFallback(result WebsiteSearchPageResult) bool {
	if result.MatchCount > 0 {
		return false
	}
	switch result.ErrorCode {
	case "blocked_address", "invalid_url", "dns_failed", "timeout", "canceled", "response_too_large":
		return false
	default:
		return true
	}
}

func (a *Service) applyWebsiteBrowserFallbacks(
	ctx context.Context,
	query string,
	targets []WebsiteSearchTarget,
	results []WebsiteSearchPageResult,
	indexes []int,
) {
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options,
		chromedp.Flag("no-proxy-server", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	allocatorContext, cancelAllocator := chromedp.NewExecAllocator(ctx, options...)
	defer cancelAllocator()
	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	defer cancelBrowser()
	if err := chromedp.Run(browserContext); err != nil {
		warning := "Browser fallback is unavailable: " + compactWebsiteError(err)
		for _, index := range indexes {
			results[index].FallbackWarning = warning
		}
		return
	}

	runBounded(browserContext, len(indexes), websiteSearchBrowserWorkerLimit, func(workerContext context.Context, position int) {
		index := indexes[position]
		document, err := fetchWebsiteWithBrowser(workerContext, targets[index].URL)
		if err != nil {
			results[index].FallbackWarning = "Browser fallback could not check this page: " + compactWebsiteError(err)
			return
		}
		results[index] = resultFromWebsiteDocument(targets[index], query, document)
	})
}

func fetchWebsiteWithBrowser(ctx context.Context, value string) (websiteDocument, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "invalid_url", Message: "The saved page URL is invalid."}
	}
	if err := validatePublicWebsiteURL(ctx, parsed); err != nil {
		return websiteDocument{}, err
	}

	pageContext, cancelPage := context.WithTimeout(ctx, websiteBrowserPageTimeout)
	defer cancelPage()
	tabContext, cancelTab := chromedp.NewContext(pageContext)
	defer cancelTab()

	guard := newWebsiteBrowserRequestGuard()
	chromedp.ListenTarget(tabContext, func(event any) {
		switch typed := event.(type) {
		case *fetch.EventRequestPaused:
			if typed.Request != nil {
				go guard.handle(tabContext, typed)
			}
		case *network.EventResponseReceived:
			if typed.Type == network.ResourceTypeDocument && typed.Response != nil {
				guard.recordMainResponse(typed.FrameID, int(typed.Response.Status), typed.Response.URL)
			}
		}
	})

	patterns := []*fetch.RequestPattern{{URLPattern: "*", RequestStage: fetch.RequestStageRequest}}
	var finalURL string
	var title string
	var visibleText string
	err = chromedp.Run(tabContext,
		network.Enable(),
		fetch.Enable().WithPatterns(patterns),
		chromedp.Navigate(parsed.String()),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(websiteBrowserSettleTime),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(fmt.Sprintf(`(() => {
			const text = document.body ? document.body.innerText : '';
			return text.slice(0, %d);
		})()`, websiteBrowserTextLimit), &visibleText),
	)
	if err != nil {
		if errors.Is(pageContext.Err(), context.DeadlineExceeded) {
			return websiteDocument{}, &websiteFetchError{Code: "browser_timeout", Message: "Browser fallback timed out."}
		}
		if blocked := guard.blockedMessage(); blocked != "" {
			return websiteDocument{}, &websiteFetchError{Code: "browser_blocked_request", Message: blocked}
		}
		return websiteDocument{}, &websiteFetchError{Code: "browser_failed", Message: "Browser fallback failed: " + compactWebsiteError(err)}
	}
	if blocked := guard.mainDocumentBlockedMessage(); blocked != "" {
		return websiteDocument{}, &websiteFetchError{Code: "browser_blocked_request", Message: blocked}
	}
	if status, _ := guard.mainResponse(); status >= http.StatusBadRequest {
		return websiteDocument{}, &websiteFetchError{
			Code:    "http_status",
			Message: fmt.Sprintf("Browser fallback received HTTP %d.", status),
		}
	}
	finalParsed, err := url.Parse(finalURL)
	if err != nil {
		return websiteDocument{}, &websiteFetchError{Code: "browser_failed", Message: "Browser fallback returned an invalid final URL."}
	}
	if err := validatePublicWebsiteURL(pageContext, finalParsed); err != nil {
		return websiteDocument{}, err
	}
	visibleText = collapseWebsiteWhitespace(visibleText)
	if visibleText == "" {
		return websiteDocument{}, &websiteFetchError{Code: "empty_content", Message: "Browser fallback did not find readable visible text."}
	}
	return websiteDocument{
		FinalURL: finalURL,
		Title:    collapseWebsiteWhitespace(title),
		Text:     visibleText,
		Method:   "browser",
	}, nil
}

type websiteBrowserRequestGuard struct {
	mu                  sync.Mutex
	blocked             string
	mainDocumentBlocked string
	mainFrameID         cdp.FrameID
	mainResponseStatus  int
	mainResponseURL     string
	requestCount        atomic.Int32
}

func newWebsiteBrowserRequestGuard() *websiteBrowserRequestGuard {
	return &websiteBrowserRequestGuard{}
}

func (g *websiteBrowserRequestGuard) handle(ctx context.Context, event *fetch.EventRequestPaused) {
	commandContext := browserCommandContext(ctx)
	if commandContext == nil {
		return
	}
	mainDocument := event.ResourceType == network.ResourceTypeDocument && g.recordDocumentFrame(event.FrameID)
	block := func(message string) {
		g.recordBlocked(message, mainDocument)
		_ = fetch.FailRequest(event.RequestID, network.ErrorReasonBlockedByClient).Do(commandContext)
	}

	if g.requestCount.Add(1) > websiteBrowserRequestLimit {
		block("Browser fallback stopped a page that made too many network requests.")
		return
	}
	method := strings.ToUpper(strings.TrimSpace(event.Request.Method))
	if method != http.MethodGet && method != http.MethodHead {
		block("Browser fallback blocked a non-read-only page request.")
		return
	}
	requestURL, err := url.Parse(event.Request.URL)
	if err != nil {
		block("Browser fallback blocked an invalid subrequest URL.")
		return
	}
	switch requestURL.Scheme {
	case "data", "blob", "about":
		_ = fetch.ContinueRequest(event.RequestID).Do(commandContext)
		return
	case "http", "https":
		validationContext, cancel := context.WithTimeout(ctx, websiteConnectTimeout)
		err = validatePublicWebsiteURL(validationContext, requestURL)
		cancel()
		if err != nil {
			block("Browser fallback blocked a local, private, or invalid page request.")
			return
		}
		_ = fetch.ContinueRequest(event.RequestID).Do(commandContext)
	default:
		block("Browser fallback blocked a non-HTTP page request.")
	}
}

func browserCommandContext(ctx context.Context) context.Context {
	chromeContext := chromedp.FromContext(ctx)
	if chromeContext == nil || chromeContext.Target == nil {
		return nil
	}
	return cdp.WithExecutor(ctx, chromeContext.Target)
}

func (g *websiteBrowserRequestGuard) recordBlocked(message string, mainDocument bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.blocked == "" {
		g.blocked = message
	}
	if mainDocument && g.mainDocumentBlocked == "" {
		g.mainDocumentBlocked = message
	}
}

func (g *websiteBrowserRequestGuard) blockedMessage() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.blocked
}

func (g *websiteBrowserRequestGuard) mainDocumentBlockedMessage() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mainDocumentBlocked
}

func (g *websiteBrowserRequestGuard) recordDocumentFrame(frameID cdp.FrameID) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.mainFrameID == "" {
		g.mainFrameID = frameID
	}
	return frameID == g.mainFrameID
}

func (g *websiteBrowserRequestGuard) recordMainResponse(frameID cdp.FrameID, status int, value string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.mainFrameID == "" || frameID != g.mainFrameID {
		return
	}
	g.mainResponseStatus = status
	g.mainResponseURL = value
}

func (g *websiteBrowserRequestGuard) mainResponse() (int, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mainResponseStatus, g.mainResponseURL
}
