package backend

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "modernc.org/sqlite"
)

// YouTubeVideo represents a single video from a channel.
type YouTubeVideo struct {
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Thumbnail    string `json:"thumbnail"`
	PublishedAt  string `json:"publishedAt"`
	ChannelID    string `json:"channelId"`
	ChannelTitle string `json:"channelTitle"`
	VideoURL     string `json:"videoUrl"`

	Views    string `json:"views,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type YouTubeCacheEntry struct {
	videos    []YouTubeVideo
	timestamp time.Time
}

// YouTubeCache caches videos per resolved UC... channel ID with a 5-minute timeout.
type YouTubeCache struct {
	videosByChannelID map[string]YouTubeCacheEntry
	mu                sync.RWMutex
}

var youtubeCache = &YouTubeCache{
	videosByChannelID: make(map[string]YouTubeCacheEntry),
}

// GetVideos returns cached videos for one channel if they are fresh.
func (c *YouTubeCache) GetVideos(channelID string) ([]YouTubeVideo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.videosByChannelID[channelID]
	if !ok {
		return nil, false
	}

	if time.Since(entry.timestamp) < 5*time.Minute && len(entry.videos) > 0 {
		return entry.videos, true
	}

	return nil, false
}

// SetVideos stores videos for one resolved channel ID.
func (c *YouTubeCache) SetVideos(channelID string, videos []YouTubeVideo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.videosByChannelID[channelID] = YouTubeCacheEntry{
		videos:    videos,
		timestamp: time.Now(),
	}
}

// Clear empties the cache.
func (c *YouTubeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.videosByChannelID = make(map[string]YouTubeCacheEntry)
}

// YouTubeCacheClear clears the video cache.
func (a *App) YouTubeCacheClear() {
	youtubeCache.Clear()
}

// ----- Handle resolution -----

var handleCache = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

// resolveChannelID takes a YouTube handle (with or without '@') and returns the channel ID (UC...).
func resolveChannelID(handle string) (string, error) {
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		return "", fmt.Errorf("empty handle")
	}

	// Check in‑memory cache
	handleCache.RLock()
	if id, ok := handleCache.m[handle]; ok {
		handleCache.RUnlock()
		return id, nil
	}
	handleCache.RUnlock()

	url := fmt.Sprintf("https://www.youtube.com/@%s", handle)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	var channelID string

	// 1. <meta itemprop="channelId" content="UC...">
	doc.Find("meta[itemprop='channelId']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists && strings.HasPrefix(content, "UC") {
			channelID = content
		}
	})
	if channelID != "" {
		goto found
	}

	// 2. <link rel="canonical" href=".../channel/UC...">
	doc.Find("link[rel='canonical']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			parts := strings.Split(href, "/")
			for _, part := range parts {
				if strings.HasPrefix(part, "UC") {
					channelID = part
					break
				}
			}
		}
	})
	if channelID != "" {
		goto found
	}

	// 3. <meta property="og:url" content=".../channel/UC...">
	doc.Find("meta[property='og:url']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			parts := strings.Split(content, "/")
			for _, part := range parts {
				if strings.HasPrefix(part, "UC") {
					channelID = part
					break
				}
			}
		}
	})
	if channelID != "" {
		goto found
	}

	// 4. Search inside <script> for JSON-like channel IDs
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptText := s.Text()
		patterns := []string{`"channelId":"`, `"channel_id":"`, `"externalChannelId":"`}
		for _, pat := range patterns {
			start := strings.Index(scriptText, pat)
			if start != -1 {
				start += len(pat)
				end := strings.Index(scriptText[start:], `"`)
				if end != -1 {
					candidate := scriptText[start : start+end]
					if strings.HasPrefix(candidate, "UC") {
						channelID = candidate
						return
					}
				}
			}
		}
	})

found:
	if channelID == "" {
		return "", fmt.Errorf("could not extract channel ID from the page")
	}

	// Store in cache
	handleCache.Lock()
	handleCache.m[handle] = channelID
	handleCache.Unlock()

	return channelID, nil
}

// ----- Video fetching -----

// GetChannelVideos returns videos for the given channel.
// It accepts either a raw channel ID (UC...) or a handle (with or without '@').
func (a *App) GetChannelVideos(channelID string) ([]YouTubeVideo, error) {
	return a.getChannelVideos(channelID, true)
}

func (a *App) getChannelVideos(channelID string, useCache bool) ([]YouTubeVideo, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channel ID or username is required")
	}

	if !strings.HasPrefix(channelID, "UC") {
		resolved, err := resolveChannelID(channelID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve channel handle: %w", err)
		}
		channelID = resolved
	}

	if useCache {
		if videos, ok := youtubeCache.GetVideos(channelID); ok {
			return videos, nil
		}
	}

	videos, err := fetchYouTubeVideos(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YouTube videos: %w", err)
	}

	if useCache {
		youtubeCache.SetVideos(channelID, videos)
	}

	return videos, nil
}

// GetChannelVideosPaginated returns the latest videos (ignores 'before' because RSS does not support pagination).
func (a *App) GetChannelVideosPaginated(channelID string, before int) ([]YouTubeVideo, error) {
	return a.GetChannelVideos(channelID)
}

// fetchYouTubeVideos downloads and parses the YouTube RSS feed.
func fetchYouTubeVideos(channelID string) ([]YouTubeVideo, error) {
	url := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", channelID)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/xml, text/xml, */*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS: %w", err)
	}

	videos := make([]YouTubeVideo, 0, len(feed.Entries))

	for _, entry := range feed.Entries {
		if entry.VideoID == "" {
			continue
		}

		video := YouTubeVideo{
			VideoID:      entry.VideoID,
			Title:        strings.TrimSpace(entry.Title),
			PublishedAt:  entry.Published,
			ChannelID:    channelID,
			ChannelTitle: strings.TrimSpace(entry.Author.Name),
			VideoURL:     fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.VideoID),
			Thumbnail:    entry.MediaGroup.Thumbnail.URL,
		}

		videos = append(videos, video)
	}

	// Reverse order (oldest first, like Telegram posts)
	for i, j := 0, len(videos)-1; i < j; i, j = i+1, j-1 {
		videos[i], videos[j] = videos[j], videos[i]
	}

	return videos, nil
}

// Atom feed structures for RSS parsing.
type AtomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []AtomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

type AtomEntry struct {
	ID        string `xml:"http://www.w3.org/2005/Atom id"`
	Title     string `xml:"http://www.w3.org/2005/Atom title"`
	Published string `xml:"http://www.w3.org/2005/Atom published"`
	Author    struct {
		Name string `xml:"http://www.w3.org/2005/Atom name"`
	} `xml:"http://www.w3.org/2005/Atom author"`
	MediaGroup struct {
		Thumbnail struct {
			URL string `xml:"url,attr"`
		} `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	} `xml:"http://search.yahoo.com/mrss/ group"`
	VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
}

// ---------- YouTube Favorites ----------

// normalizeYouTubeChannelID trims spaces; no further validation.
func normalizeYouTubeChannelID(id string) string {
	id = strings.TrimSpace(id)
	return id
}

func normalizeYouTubeUsername(username string) string {
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	return username
}

// AddYouTubeFavorite adds a channel to favorites.
// It accepts a handle and resolves it before storing the channel ID.
func (a *App) AddYouTubeFavorite(channelID string) ([]string, error) {
	channelID = normalizeYouTubeChannelID(channelID)
	if channelID == "" {
		return a.ListYouTubeFavorites()
	}

	username := ""
	if !strings.HasPrefix(channelID, "UC") {
		username = normalizeYouTubeUsername(channelID)
	}

	// Resolve handle to ID if needed
	if !strings.HasPrefix(channelID, "UC") {
		resolved, err := resolveChannelID(channelID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve channel handle: %w", err)
		}
		channelID = resolved
	}

	_, err := a.db.Exec(
		`INSERT INTO youtube_favorites (channel_id, username)
		VALUES (?, NULLIF(?, ''))
		ON CONFLICT(channel_id) DO UPDATE SET
			username = COALESCE(NULLIF(excluded.username, ''), youtube_favorites.username)`,
		channelID, username,
	)
	if err != nil {
		return nil, err
	}

	return a.ListYouTubeFavorites()
}

// RemoveYouTubeFavorite removes a channel from favorites.
func (a *App) RemoveYouTubeFavorite(channelID string) ([]string, error) {
	channelID = normalizeYouTubeChannelID(channelID)
	if channelID == "" {
		return a.ListYouTubeFavorites()
	}
	if !strings.HasPrefix(channelID, "UC") {
		resolved, err := resolveChannelID(channelID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve channel handle: %w", err)
		}
		channelID = resolved
	}

	_, err := a.db.Exec(`DELETE FROM youtube_favorites WHERE channel_id = ?`, channelID)
	if err != nil {
		return nil, err
	}

	return a.ListYouTubeFavorites()
}

// ListYouTubeFavorites returns the list of favorite channel IDs.
func (a *App) ListYouTubeFavorites() ([]string, error) {
	rows, err := a.db.Query(`SELECT channel_id FROM youtube_favorites ORDER BY added_at ASC`)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			favorites = append(favorites, id)
		}
	}
	return favorites, nil
}

func (a *App) AssignYouTubeFavoriteCategory(channelID string, categoryID int) error {
	channelID = normalizeYouTubeChannelID(channelID)
	if channelID == "" {
		return fmt.Errorf("youtube channel is required")
	}

	if !strings.HasPrefix(channelID, "UC") {
		resolved, err := resolveChannelID(channelID)
		if err != nil {
			return fmt.Errorf("failed to resolve channel handle: %w", err)
		}
		channelID = resolved
	}

	if categoryID <= 0 {
		return fmt.Errorf("invalid category ID")
	}

	if err := a.ensureFavoriteCategoryExists(categoryID, favoriteSourceYouTube); err != nil {
		return err
	}

	_, err := a.db.Exec(
		`UPDATE youtube_favorites SET category_id = ? WHERE channel_id = ?`,
		categoryID, channelID,
	)
	if err != nil {
		return fmt.Errorf("failed to assign category: %w", err)
	}
	return nil
}

func (a *App) ListYouTubeFavoritesWithCategories() ([]FavoriteChannel, error) {
	rows, err := a.db.Query(`SELECT channel_id, COALESCE(username, ''), category_id
		FROM youtube_favorites
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
		if err := rows.Scan(&channel.ChannelID, &channel.Username, &categoryID); err != nil {
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
