package backend

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "modernc.org/sqlite"
)

type TelegramPost struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
	Date   string   `json:"date"`
	Views  string   `json:"views"`
	PostID string   `json:"postId"`
}

type TelegramCache struct {
	posts     []TelegramPost
	timestamp time.Time
	mu        sync.RWMutex
}

var telegramCache = &TelegramCache{}

func (a *App) GetChannelPosts(channelUsername string) ([]TelegramPost, error) {
	if posts, ok := telegramCache.GetPosts(); ok {
		return posts, nil
	}

	channelUsername = strings.TrimSpace(channelUsername)
	if channelUsername == "" {
		return nil, fmt.Errorf("channel username is required")
	}

	channelUsername = strings.TrimPrefix(channelUsername, "@")

	url := fmt.Sprintf("https://t.me/s/%s", channelUsername)

	posts, err := fetchTelegramPosts(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel posts: %w", err)
	}

	telegramCache.SetPosts(posts)

	return posts, nil
}

func (a *App) GetChannelPostsPaginated(channelUsername string, before int) ([]TelegramPost, error) {
	channelUsername = strings.TrimSpace(strings.TrimPrefix(channelUsername, "@"))
	if channelUsername == "" {
		return nil, fmt.Errorf("channel username is required")
	}

	var url string
	if before == 0 {
		url = fmt.Sprintf("https://t.me/s/%s", channelUsername)
	} else {
		url = fmt.Sprintf("https://t.me/s/%s?before=%d", channelUsername, before)
	}

	return fetchTelegramPosts(url)
}

func fetchTelegramPosts(url string) ([]TelegramPost, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
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

		postID, exists := s.Attr("data-post")
		if exists {
			post.PostID = postID
		}

		if post.Text != "" || len(post.Images) > 0 {
			posts = append(posts, post)
		}
	})
	for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
		posts[i], posts[j] = posts[j], posts[i]
	}
	return posts, nil
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

func (c *TelegramCache) GetPosts() ([]TelegramPost, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Since(c.timestamp) < 5*time.Minute && len(c.posts) > 0 {
		return c.posts, true
	}
	return nil, false
}

func (c *TelegramCache) SetPosts(posts []TelegramPost) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.posts = posts
	c.timestamp = time.Now()
}

func (c *TelegramCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.posts = nil
	c.timestamp = time.Time{}
}

func (a *App) TelegramCacheClear() {
	telegramCache.Clear()
}

func normalizeTelegramUsername(username string) string {
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	return strings.ToLower(username)
}

// AddTelegramFavorite сохраняет канал в списке избранных
func (a *App) AddTelegramFavorite(username string) ([]string, error) {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return a.ListTelegramFavorites()
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
	username = normalizeTelegramUsername(username)
	if username == "" {
		return a.ListTelegramFavorites()
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
		if err := rows.Scan(&username); err == nil {
			favorites = append(favorites, username)
		}
	}

	return favorites, nil
}

func (a *App) AssignTelegramFavoriteCategory(username string, categoryID int) error {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
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
