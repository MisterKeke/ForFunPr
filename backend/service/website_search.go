package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

const (
	maximumWebsiteSearchTargets     = 50
	maximumWebsiteSearchURLBytes    = 4096
	maximumWebsiteSearchQueryRunes  = 256
	maximumWebsiteSearchSnippets    = 3
	websiteSearchSnippetContext     = 90
	websiteSearchWorkerLimit        = 6
	websiteSearchBrowserWorkerLimit = 2
	websiteSearchHistoryLimit       = 12
	websiteSearchStoredRunLimit     = 50
	websiteSearchOverallTimeout     = 90 * time.Second
)

type WebsiteSearchTarget struct {
	ID            int    `json:"id"`
	URL           string `json:"url"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type WebsiteSearchHistoryItem struct {
	ID                 int    `json:"id"`
	Query              string `json:"query"`
	UseBrowserFallback bool   `json:"use_browser_fallback"`
	Checked            int    `json:"checked"`
	Found              int    `json:"found"`
	Failed             int    `json:"failed"`
	CreatedAt          string `json:"created_at"`
}

type WebsiteSearchState struct {
	Targets        []WebsiteSearchTarget      `json:"targets"`
	RecentSearches []WebsiteSearchHistoryItem `json:"recent_searches"`
}

type WebsiteSearchRequest struct {
	Query              string `json:"query"`
	UseBrowserFallback bool   `json:"use_browser_fallback"`
}

type WebsiteSearchPageResult struct {
	TargetID        int      `json:"target_id"`
	OriginalURL     string   `json:"original_url"`
	FinalURL        string   `json:"final_url,omitempty"`
	Title           string   `json:"title,omitempty"`
	Hostname        string   `json:"hostname,omitempty"`
	MatchCount      int      `json:"match_count"`
	Snippets        []string `json:"snippets"`
	OpenURL         string   `json:"open_url,omitempty"`
	FetchMethod     string   `json:"fetch_method,omitempty"`
	ErrorCode       string   `json:"error_code,omitempty"`
	ErrorMessage    string   `json:"error_message,omitempty"`
	FallbackWarning string   `json:"fallback_warning,omitempty"`
	CheckedAt       string   `json:"checked_at"`
}

type WebsiteSearchResponse struct {
	RunID              int                       `json:"run_id"`
	Query              string                    `json:"query"`
	UseBrowserFallback bool                      `json:"use_browser_fallback"`
	Checked            int                       `json:"checked"`
	Found              int                       `json:"found"`
	Failed             int                       `json:"failed"`
	Results            []WebsiteSearchPageResult `json:"results"`
	CreatedAt          string                    `json:"created_at"`
}

type websiteDocument struct {
	FinalURL string
	Title    string
	Text     string
	Method   string
}

func (a *Service) GetWebsiteSearchStateContext(ctx context.Context) (WebsiteSearchState, error) {
	targets, err := a.listWebsiteSearchTargetsContext(ctx)
	if err != nil {
		return WebsiteSearchState{}, err
	}
	history, err := a.listWebsiteSearchHistoryContext(ctx, websiteSearchHistoryLimit)
	if err != nil {
		return WebsiteSearchState{}, err
	}
	return WebsiteSearchState{Targets: targets, RecentSearches: history}, nil
}

func (a *Service) AddWebsiteSearchTargetContext(ctx context.Context, value string) (WebsiteSearchTarget, error) {
	displayURL, normalizedURL, err := normalizeWebsiteSearchURL(value)
	if err != nil {
		return WebsiteSearchTarget{}, err
	}

	var existingID int
	err = a.db.QueryRowContext(ctx, `
		SELECT id FROM website_search_targets WHERE normalized_url = ?
	`, normalizedURL).Scan(&existingID)
	if err == nil {
		return WebsiteSearchTarget{}, &ConflictError{Resource: "website search URL", Message: "That URL is already in Website Search."}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return WebsiteSearchTarget{}, fmt.Errorf("check website search URL: %w", err)
	}
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM website_search_targets`).Scan(&count); err != nil {
		return WebsiteSearchTarget{}, fmt.Errorf("count website search targets: %w", err)
	}
	if count >= maximumWebsiteSearchTargets {
		return WebsiteSearchTarget{}, &ValidationError{
			Field:   "url",
			Message: fmt.Sprintf("Website Search supports at most %d saved URLs.", maximumWebsiteSearchTargets),
		}
	}

	result, err := a.db.ExecContext(ctx, `
		INSERT INTO website_search_targets (url, normalized_url) VALUES (?, ?)
	`, displayURL, normalizedURL)
	if err != nil {
		return WebsiteSearchTarget{}, fmt.Errorf("save website search URL: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WebsiteSearchTarget{}, fmt.Errorf("read saved website search URL id: %w", err)
	}
	return a.getWebsiteSearchTargetContext(ctx, int(id))
}

func (a *Service) DeleteWebsiteSearchTargetContext(ctx context.Context, id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "website search URL id must be positive"}
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM website_search_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete website search URL: %w", err)
	}
	return requireSingleMutation(result, "delete website search URL", "website search URL", false)
}

func (a *Service) SearchWebsitesContext(ctx context.Context, request WebsiteSearchRequest) (WebsiteSearchResponse, error) {
	query, err := normalizeWebsiteSearchQuery(request.Query)
	if err != nil {
		return WebsiteSearchResponse{}, err
	}
	targets, err := a.listWebsiteSearchTargetsContext(ctx)
	if err != nil {
		return WebsiteSearchResponse{}, err
	}
	if len(targets) == 0 {
		return WebsiteSearchResponse{}, &ValidationError{Field: "urls", Message: "Add at least one URL before searching."}
	}

	searchContext, cancel := context.WithTimeout(ctx, websiteSearchOverallTimeout)
	defer cancel()

	results := make([]WebsiteSearchPageResult, len(targets))
	runBounded(searchContext, len(targets), websiteSearchWorkerLimit, func(workerContext context.Context, index int) {
		results[index] = a.searchWebsiteTargetHTTP(workerContext, targets[index], query)
	})

	if request.UseBrowserFallback && searchContext.Err() == nil {
		indexes := make([]int, 0, len(results))
		for index := range results {
			if shouldTryWebsiteBrowserFallback(results[index]) {
				indexes = append(indexes, index)
			}
		}
		if len(indexes) > 0 {
			a.applyWebsiteBrowserFallbacks(searchContext, query, targets, results, indexes)
		}
	}

	response := summarizeWebsiteSearch(query, request.UseBrowserFallback, results)
	if err := a.storeWebsiteSearchRunContext(ctx, &response); err != nil {
		return WebsiteSearchResponse{}, err
	}
	return response, nil
}

func (a *Service) GetWebsiteSearchRunContext(ctx context.Context, id int) (WebsiteSearchResponse, error) {
	if id <= 0 {
		return WebsiteSearchResponse{}, &ValidationError{Field: "id", Message: "website search run id must be positive"}
	}
	var response WebsiteSearchResponse
	var useBrowser int
	if err := a.db.QueryRowContext(ctx, `
		SELECT id, query, use_browser_fallback, checked_count, found_count, failed_count, created_at
		FROM website_search_runs WHERE id = ?
	`, id).Scan(
		&response.RunID,
		&response.Query,
		&useBrowser,
		&response.Checked,
		&response.Found,
		&response.Failed,
		&response.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WebsiteSearchResponse{}, &NotFoundError{Resource: "website search run", Key: fmt.Sprint(id)}
		}
		return WebsiteSearchResponse{}, fmt.Errorf("load website search run: %w", err)
	}
	response.UseBrowserFallback = useBrowser != 0

	rows, err := a.db.QueryContext(ctx, `
		SELECT COALESCE(target_id, 0), original_url, final_url, title, hostname,
		       match_count, snippets_json, open_url, fetch_method, error_code,
		       error_message, fallback_warning, checked_at
		FROM website_search_results
		WHERE run_id = ?
		ORDER BY id
	`, id)
	if err != nil {
		return WebsiteSearchResponse{}, fmt.Errorf("list website search results: %w", err)
	}
	defer rows.Close()
	response.Results = []WebsiteSearchPageResult{}
	for rows.Next() {
		var item WebsiteSearchPageResult
		var snippetsJSON string
		if err := rows.Scan(
			&item.TargetID,
			&item.OriginalURL,
			&item.FinalURL,
			&item.Title,
			&item.Hostname,
			&item.MatchCount,
			&snippetsJSON,
			&item.OpenURL,
			&item.FetchMethod,
			&item.ErrorCode,
			&item.ErrorMessage,
			&item.FallbackWarning,
			&item.CheckedAt,
		); err != nil {
			return WebsiteSearchResponse{}, fmt.Errorf("scan website search result: %w", err)
		}
		item.Snippets = []string{}
		if err := json.Unmarshal([]byte(snippetsJSON), &item.Snippets); err != nil {
			return WebsiteSearchResponse{}, fmt.Errorf("decode website search snippets: %w", err)
		}
		response.Results = append(response.Results, item)
	}
	if err := rows.Err(); err != nil {
		return WebsiteSearchResponse{}, fmt.Errorf("iterate website search results: %w", err)
	}
	return response, nil
}

func (a *Service) listWebsiteSearchTargetsContext(ctx context.Context) ([]WebsiteSearchTarget, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, url, last_checked_at, created_at
		FROM website_search_targets
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list website search URLs: %w", err)
	}
	defer rows.Close()
	targets := []WebsiteSearchTarget{}
	for rows.Next() {
		var target WebsiteSearchTarget
		var lastChecked sql.NullString
		if err := rows.Scan(&target.ID, &target.URL, &lastChecked, &target.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan website search URL: %w", err)
		}
		if lastChecked.Valid {
			target.LastCheckedAt = lastChecked.String
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate website search URLs: %w", err)
	}
	return targets, nil
}

func (a *Service) getWebsiteSearchTargetContext(ctx context.Context, id int) (WebsiteSearchTarget, error) {
	var target WebsiteSearchTarget
	var lastChecked sql.NullString
	if err := a.db.QueryRowContext(ctx, `
		SELECT id, url, last_checked_at, created_at
		FROM website_search_targets WHERE id = ?
	`, id).Scan(&target.ID, &target.URL, &lastChecked, &target.CreatedAt); err != nil {
		return WebsiteSearchTarget{}, fmt.Errorf("load website search URL: %w", err)
	}
	if lastChecked.Valid {
		target.LastCheckedAt = lastChecked.String
	}
	return target, nil
}

func (a *Service) listWebsiteSearchHistoryContext(ctx context.Context, limit int) ([]WebsiteSearchHistoryItem, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, query, use_browser_fallback, checked_count, found_count, failed_count, created_at
		FROM website_search_runs
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list website search history: %w", err)
	}
	defer rows.Close()
	history := []WebsiteSearchHistoryItem{}
	for rows.Next() {
		var item WebsiteSearchHistoryItem
		var useBrowser int
		if err := rows.Scan(
			&item.ID,
			&item.Query,
			&useBrowser,
			&item.Checked,
			&item.Found,
			&item.Failed,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan website search history: %w", err)
		}
		item.UseBrowserFallback = useBrowser != 0
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate website search history: %w", err)
	}
	return history, nil
}

func (a *Service) storeWebsiteSearchRunContext(ctx context.Context, response *WebsiteSearchResponse) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin website search history write: %w", err)
	}
	defer tx.Rollback()

	runResult, err := tx.ExecContext(ctx, `
		INSERT INTO website_search_runs (
			query, use_browser_fallback, checked_count, found_count, failed_count
		) VALUES (?, ?, ?, ?, ?)
	`, response.Query, response.UseBrowserFallback, response.Checked, response.Found, response.Failed)
	if err != nil {
		return fmt.Errorf("save website search run: %w", err)
	}
	runID, err := runResult.LastInsertId()
	if err != nil {
		return fmt.Errorf("read website search run id: %w", err)
	}
	response.RunID = int(runID)

	for _, item := range response.Results {
		snippetsJSON, err := json.Marshal(item.Snippets)
		if err != nil {
			return fmt.Errorf("encode website search snippets: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO website_search_results (
				run_id, target_id, original_url, final_url, title, hostname,
				match_count, snippets_json, open_url, fetch_method, error_code,
				error_message, fallback_warning, checked_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, runID, nullablePositiveID(item.TargetID), item.OriginalURL, item.FinalURL,
			item.Title, item.Hostname, item.MatchCount, string(snippetsJSON), item.OpenURL,
			item.FetchMethod, item.ErrorCode, item.ErrorMessage, item.FallbackWarning,
			item.CheckedAt); err != nil {
			return fmt.Errorf("save website search result: %w", err)
		}
		if item.TargetID > 0 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE website_search_targets SET last_checked_at = ? WHERE id = ?
			`, item.CheckedAt, item.TargetID); err != nil {
				return fmt.Errorf("update website search check time: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM website_search_runs
		WHERE id NOT IN (
			SELECT id FROM website_search_runs ORDER BY created_at DESC, id DESC LIMIT ?
		)
	`, websiteSearchStoredRunLimit); err != nil {
		return fmt.Errorf("prune website search history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit website search history: %w", err)
	}
	return nil
}

func normalizeWebsiteSearchURL(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", &ValidationError{Field: "url", Message: "Enter an HTTP or HTTPS URL."}
	}
	if len(value) > maximumWebsiteSearchURLBytes {
		return "", "", &ValidationError{Field: "url", Message: "Website Search URLs must be 4096 bytes or fewer."}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", "", &ValidationError{Field: "url", Message: "Website Search URLs cannot contain control characters."}
		}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", "", &ValidationError{Field: "url", Message: "Enter a valid HTTP or HTTPS URL."}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", &ValidationError{Field: "url", Message: "Website Search URLs must use HTTP or HTTPS."}
	}
	if parsed.User != nil {
		return "", "", &ValidationError{Field: "url", Message: "Website Search URLs cannot contain credentials."}
	}
	if parsed.Hostname() == "" {
		return "", "", &ValidationError{Field: "url", Message: "Website Search URLs must include a hostname."}
	}
	if err := rejectObviousInternalHostname(parsed.Hostname()); err != nil {
		return "", "", err
	}
	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	displayURL := parsed.String()
	normalized := *parsed
	normalized.Fragment = ""
	normalized.RawFragment = ""
	if normalized.Path == "" {
		normalized.Path = "/"
	}
	return displayURL, normalized.String(), nil
}

func normalizeWebsiteSearchQuery(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &ValidationError{Field: "query", Message: "Enter a word or phrase to search for."}
	}
	if !utf8.ValidString(value) {
		return "", &ValidationError{Field: "query", Message: "The search text must be valid UTF-8."}
	}
	if utf8.RuneCountInString(value) > maximumWebsiteSearchQueryRunes {
		return "", &ValidationError{Field: "query", Message: "Search text must be 256 characters or fewer."}
	}
	return value, nil
}

func extractWebsiteHTML(body []byte) (title string, visibleText string, err error) {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("HTML could not be parsed: %w", err)
	}
	title = collapseWebsiteWhitespace(document.Find("title").First().Text())
	if title == "" {
		title, _ = document.Find(`meta[property="og:title"]`).First().Attr("content")
		title = collapseWebsiteWhitespace(title)
	}
	document.Find("script, style, noscript, template, svg, canvas, head, [hidden], [aria-hidden='true']").Remove()
	selection := document.Find("body").First()
	if selection.Length() == 0 {
		selection = document.Selection
	}
	visibleText = collapseWebsiteWhitespace(selection.Text())
	return title, visibleText, nil
}

func collapseWebsiteWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func findWebsiteMatches(text string, query string) (int, []string) {
	textRunes := []rune(text)
	queryRunes := []rune(query)
	if len(textRunes) == 0 || len(queryRunes) == 0 || len(queryRunes) > len(textRunes) {
		return 0, []string{}
	}
	foldedText := foldWebsiteRunes(textRunes)
	foldedQuery := foldWebsiteRunes(queryRunes)
	starts := make([]int, 0, maximumWebsiteSearchSnippets)
	count := 0
	for index := 0; index+len(foldedQuery) <= len(foldedText); {
		if equalWebsiteRunes(foldedText[index:index+len(foldedQuery)], foldedQuery) {
			count++
			if len(starts) < maximumWebsiteSearchSnippets &&
				(len(starts) == 0 || index-starts[len(starts)-1] > websiteSearchSnippetContext) {
				starts = append(starts, index)
			}
			index += len(foldedQuery)
			continue
		}
		index++
	}
	snippets := make([]string, 0, len(starts))
	for _, start := range starts {
		snippets = append(snippets, websiteSearchSnippet(textRunes, start, start+len(queryRunes)))
	}
	return count, snippets
}

func foldWebsiteRunes(value []rune) []rune {
	folded := make([]rune, len(value))
	for index, character := range value {
		folded[index] = unicode.ToLower(character)
	}
	return folded
}

func equalWebsiteRunes(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func websiteSearchSnippet(text []rune, matchStart, matchEnd int) string {
	start := max(0, matchStart-websiteSearchSnippetContext)
	end := min(len(text), matchEnd+websiteSearchSnippetContext)
	for start > 0 && !unicode.IsSpace(text[start-1]) && matchStart-start < websiteSearchSnippetContext+24 {
		start--
	}
	for end < len(text) && !unicode.IsSpace(text[end]) && end-matchEnd < websiteSearchSnippetContext+24 {
		end++
	}
	snippet := strings.TrimSpace(string(text[start:end]))
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet += "…"
	}
	return snippet
}

func resultFromWebsiteDocument(target WebsiteSearchTarget, query string, document websiteDocument) WebsiteSearchPageResult {
	matchCount, snippets := findWebsiteMatches(document.Text, query)
	parsed, _ := url.Parse(document.FinalURL)
	title := document.Title
	if title == "" && parsed != nil {
		title = parsed.Hostname()
	}
	result := WebsiteSearchPageResult{
		TargetID:    target.ID,
		OriginalURL: target.URL,
		FinalURL:    document.FinalURL,
		Title:       title,
		MatchCount:  matchCount,
		Snippets:    snippets,
		FetchMethod: document.Method,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if parsed != nil {
		result.Hostname = parsed.Hostname()
	}
	if matchCount > 0 {
		result.OpenURL = websiteTextFragmentURL(document.FinalURL, query)
	}
	return result
}

func websiteTextFragmentURL(value, query string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	parsed.Fragment = ""
	parsed.RawFragment = ""
	encodedQuery := strings.ReplaceAll(url.QueryEscape(query), "+", "%20")
	return parsed.String() + "#:~:text=" + encodedQuery
}

func summarizeWebsiteSearch(query string, useBrowser bool, results []WebsiteSearchPageResult) WebsiteSearchResponse {
	response := WebsiteSearchResponse{
		Query:              query,
		UseBrowserFallback: useBrowser,
		Checked:            len(results),
		Results:            results,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	for _, result := range results {
		if result.MatchCount > 0 {
			response.Found++
		}
		if result.ErrorCode != "" {
			response.Failed++
		}
	}
	return response
}

func nullablePositiveID(id int) any {
	if id <= 0 {
		return nil
	}
	return id
}
