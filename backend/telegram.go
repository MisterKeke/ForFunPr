package backend

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type TelegramPost struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
	Date   string   `json:"date"`
	Views  string   `json:"views"`
	PostID string   `json:"postId"`
}

type telegramCacheEntry struct {
	posts     []TelegramPost
	timestamp time.Time
}

type telegramPostsCache struct {
	postsByUsername map[string]telegramCacheEntry
	mu              sync.RWMutex
}

var telegramCache = &telegramPostsCache{
	postsByUsername: make(map[string]telegramCacheEntry),
}

func (a *App) GetChannelPosts(channelUsername string) ([]TelegramPost, error) {
	return a.getChannelPosts(a.requestContext(), channelUsername, true)
}

func (a *App) getChannelPosts(ctx context.Context, channelUsername string, useCache bool) ([]TelegramPost, error) {
	channelUsername = normalizeTelegramUsername(channelUsername)
	if channelUsername == "" {
		return nil, fmt.Errorf("invalid Telegram channel username")
	}

	if useCache {
		if posts, ok := telegramCache.getPosts(channelUsername); ok {
			return posts, nil
		}
	}

	posts, err := a.fetchTelegramPosts(ctx, channelUsername, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel posts: %w", err)
	}

	if useCache {
		telegramCache.setPosts(channelUsername, posts)
	}

	return posts, nil
}

func (a *App) GetChannelPostsPaginated(channelUsername string, before int) ([]TelegramPost, error) {
	channelUsername = normalizeTelegramUsername(channelUsername)
	if channelUsername == "" {
		return nil, fmt.Errorf("invalid Telegram channel username")
	}
	if before < 0 {
		return nil, fmt.Errorf("Telegram pagination cursor cannot be negative")
	}

	return a.fetchTelegramPosts(a.requestContext(), channelUsername, before)
}

func (a *App) fetchTelegramPosts(ctx context.Context, username string, before int) ([]TelegramPost, error) {
	body, _, err := a.httpClient.get(
		ctx,
		providerTelegram,
		telegramPostsURL(username, before),
		http.Header{
			"User-Agent":      []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			"Accept":          []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
			"Accept-Language": []string{"en-US,en;q=0.5"},
		},
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}

	posts, err := parseTelegramPosts(body)
	if err != nil {
		return nil, fmt.Errorf("Telegram response could not be parsed: %w", err)
	}
	return posts, nil
}

func telegramPostsURL(username string, before int) *url.URL {
	endpoint := (&url.URL{
		Scheme: "https",
		Host:   "t.me",
	}).JoinPath("s", username)
	if before > 0 {
		query := endpoint.Query()
		query.Set("before", fmt.Sprintf("%d", before))
		endpoint.RawQuery = query.Encode()
	}
	return endpoint
}

func parseTelegramPosts(body []byte) ([]TelegramPost, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var posts []TelegramPost

	doc.Find(".tgme_widget_message_wrap").Each(func(i int, s *goquery.Selection) {
		post := TelegramPost{}

		textElement := s.Find(".tgme_widget_message_text")
		post.Text = strings.TrimSpace(textElement.Text())

		s.Find(".tgme_widget_message_photo_wrap").Each(func(j int, img *goquery.Selection) {
			style, exists := img.Attr("style")
			if exists {
				url := extractImageURL(style)
				if url != "" {
					post.Images = append(post.Images, url)
				}
			}
		})

		s.Find(".tgme_widget_message_photo img").Each(func(j int, img *goquery.Selection) {
			src, exists := img.Attr("src")
			if exists && src != "" {
				if !strings.Contains(src, "data:image") {
					post.Images = append(post.Images, src)
				}
			}
		})

		dateElement := s.Find(".tgme_widget_message_date time")
		if dateElement.Length() > 0 {
			post.Date, _ = dateElement.Attr("datetime")
		}

		viewsElement := s.Find(".tgme_widget_message_views")
		post.Views = strings.TrimSpace(viewsElement.Text())

		post.PostID = extractTelegramPostID(s)

		if post.Text != "" || len(post.Images) > 0 || post.PostID != "" || post.Date != "" {
			posts = append(posts, post)
		}
	})
	for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
		posts[i], posts[j] = posts[j], posts[i]
	}
	return posts, nil
}

func extractTelegramPostID(s *goquery.Selection) string {
	if postID, exists := s.Attr("data-post"); exists {
		return strings.TrimSpace(postID)
	}

	if postID, exists := s.Find(".tgme_widget_message").First().Attr("data-post"); exists {
		return strings.TrimSpace(postID)
	}

	if href, exists := s.Find(".tgme_widget_message_date").First().Attr("href"); exists {
		return telegramPostIDFromURL(href)
	}

	return ""
}

func telegramPostIDFromURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if index := strings.Index(value, "?"); index >= 0 {
		value = value[:index]
	}
	if index := strings.Index(value, "#"); index >= 0 {
		value = value[:index]
	}
	if index := strings.Index(value, "t.me/"); index >= 0 {
		value = value[index+len("t.me/"):]
	}

	value = strings.Trim(value, "/")
	value = strings.TrimPrefix(value, "s/")
	if strings.Count(value, "/") < 1 {
		return ""
	}
	return value
}

func extractImageURL(style string) string {
	start := strings.Index(style, "background-image:")
	if start == -1 {
		return ""
	}

	urlStart := strings.Index(style[start:], "url(")
	if urlStart == -1 {
		return ""
	}
	urlStart += start + 4

	for urlStart < len(style) && style[urlStart] == ' ' {
		urlStart++
	}

	hasQuote := false
	if urlStart < len(style) && (style[urlStart] == '\'' || style[urlStart] == '"') {
		hasQuote = true
		urlStart++
	}

	urlEnd := urlStart
	if hasQuote {

		for urlEnd < len(style) && style[urlEnd] != '\'' && style[urlEnd] != '"' {
			urlEnd++
		}
	} else {

		for urlEnd < len(style) && style[urlEnd] != ')' {
			urlEnd++
		}
	}

	if urlEnd <= urlStart || urlStart >= len(style) {
		return ""
	}

	return style[urlStart:urlEnd]
}

func (c *telegramPostsCache) getPosts(username string) ([]TelegramPost, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.postsByUsername[username]
	if !ok {
		return nil, false
	}

	if time.Since(entry.timestamp) < favoriteCacheTTL && len(entry.posts) > 0 {
		return entry.posts, true
	}

	return nil, false
}

func (c *telegramPostsCache) setPosts(username string, posts []TelegramPost) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.postsByUsername[username] = telegramCacheEntry{
		posts:     posts,
		timestamp: time.Now(),
	}
}

func (c *telegramPostsCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.postsByUsername = make(map[string]telegramCacheEntry)
}

func (a *App) TelegramCacheClear() {
	telegramCache.clear()
}

// AddTelegramFavorite сохраняет канал в списке избранных
func (a *App) AddTelegramFavorite(username string) ([]string, error) {
	if strings.TrimSpace(username) == "" {
		return a.ListTelegramFavorites()
	}
	username = normalizeTelegramUsername(username)
	if username == "" {
		return nil, fmt.Errorf("invalid Telegram channel username")
	}

	_, err := a.db.Exec(
		`INSERT OR IGNORE INTO telegram_favorites (username) VALUES (?)`,
		username,
	)
	if err != nil {
		return nil, err
	}

	return a.ListTelegramFavorites()
}

func (a *App) RemoveTelegramFavorite(username string) ([]string, error) {
	if strings.TrimSpace(username) == "" {
		return a.ListTelegramFavorites()
	}
	username = normalizeTelegramUsername(username)
	if username == "" {
		return nil, fmt.Errorf("invalid Telegram channel username")
	}

	_, err := a.db.Exec(`DELETE FROM telegram_favorites WHERE username = ?`, username)
	if err != nil {
		return nil, err
	}

	return a.ListTelegramFavorites()
}

func (a *App) ListTelegramFavorites() ([]string, error) {
	rows, err := a.db.Query(`SELECT username FROM telegram_favorites ORDER BY added_at ASC`)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return []string{}, err
		}
		favorites = append(favorites, username)
	}

	if err := rows.Err(); err != nil {
		return []string{}, err
	}

	return favorites, nil
}

func (a *App) AssignTelegramFavoriteCategory(username string, categoryID int) error {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return fmt.Errorf("invalid Telegram channel username")
	}
	if categoryID <= 0 {
		return fmt.Errorf("invalid category ID")
	}
	ensureErr := a.ensureFavoriteCategoryExists(categoryID, favoriteSourceTelegram)
	if ensureErr != nil {
		return ensureErr
	}

	_, err := a.db.Exec(
		`UPDATE telegram_favorites SET category_id = ? WHERE username = ?`,
		categoryID, username,
	)
	if err != nil {
		return fmt.Errorf("failed to assign category: %w", err)
	}
	return nil
}

func (a *App) ListTelegramFavoritesWithCategories() ([]FavoriteChannel, error) {
	rows, err := a.db.Query(`SELECT username, category_id
		FROM telegram_favorites
		ORDER BY added_at ASC
	`)
	if err != nil {
		return []FavoriteChannel{}, err
	}
	defer rows.Close()

	favorites := []FavoriteChannel{}
	for rows.Next() {
		var channel FavoriteChannel
		var categoryID sql.NullInt64
		if err := rows.Scan(&channel.Username, &categoryID); err != nil {
			return []FavoriteChannel{}, err
		}

		if categoryID.Valid {
			value := int(categoryID.Int64)
			channel.CategoryID = &value
		}

		favorites = append(favorites, channel)
	}

	return favorites, rows.Err()
}
