package backend

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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

type youtubeCacheEntry struct {
	videos    []YouTubeVideo
	timestamp time.Time
}

// youtubeVideosCache caches videos per resolved UC... channel ID.
type youtubeVideosCache struct {
	videosByChannelID map[string]youtubeCacheEntry
	mu                sync.RWMutex
}

var youtubeCache = &youtubeVideosCache{
	videosByChannelID: make(map[string]youtubeCacheEntry),
}

func (c *youtubeVideosCache) getVideos(channelID string) ([]YouTubeVideo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.videosByChannelID[channelID]
	if !ok {
		return nil, false
	}

	if time.Since(entry.timestamp) < favoriteCacheTTL && len(entry.videos) > 0 {
		return entry.videos, true
	}

	return nil, false
}

func (c *youtubeVideosCache) setVideos(channelID string, videos []YouTubeVideo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.videosByChannelID[channelID] = youtubeCacheEntry{
		videos:    videos,
		timestamp: time.Now(),
	}
}

func (c *youtubeVideosCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.videosByChannelID = make(map[string]youtubeCacheEntry)
}

// YouTubeCacheClear clears the video cache.
func (a *App) YouTubeCacheClear() {
	youtubeCache.clear()
}

var handleCache = struct {
	sync.RWMutex
	m map[string]string
}{m: make(map[string]string)}

// resolveChannelID fetches and parses the public channel page for one valid
// YouTube handle. The result is always a validated UC... channel ID.
func (a *App) resolveChannelID(ctx context.Context, handle string) (string, error) {
	handle = normalizeYouTubeUsername(handle)
	if handle == "" {
		return "", fmt.Errorf("invalid YouTube handle")
	}

	cacheKey := strings.ToLower(handle)
	handleCache.RLock()
	if channelID, ok := handleCache.m[cacheKey]; ok {
		handleCache.RUnlock()
		return channelID, nil
	}
	handleCache.RUnlock()

	body, _, err := a.httpClient.get(
		ctx,
		providerYouTube,
		youtubeHandleURL(handle),
		http.Header{
			"User-Agent":      []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			"Accept-Language": []string{"en-US,en;q=0.9"},
		},
		http.StatusOK,
	)
	if err != nil {
		return "", err
	}

	channelID, err := parseYouTubeChannelIDPage(body)
	if err != nil {
		return "", err
	}

	handleCache.Lock()
	handleCache.m[cacheKey] = channelID
	handleCache.Unlock()

	return channelID, nil
}

func youtubeHandleURL(handle string) *url.URL {
	return (&url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
	}).JoinPath("@" + handle)
}

func parseYouTubeChannelIDPage(body []byte) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	channelID := ""
	setChannelID := func(candidate string) {
		if channelID != "" {
			return
		}
		channelID = normalizeYouTubeChannelID(candidate)
	}

	doc.Find("meta[itemprop='channelId']").Each(func(_ int, selection *goquery.Selection) {
		if content, ok := selection.Attr("content"); ok {
			setChannelID(content)
		}
	})
	doc.Find("link[rel='canonical']").Each(func(_ int, selection *goquery.Selection) {
		if href, ok := selection.Attr("href"); ok {
			setChannelID(channelIDFromYouTubePageURL(href))
		}
	})
	doc.Find("meta[property='og:url']").Each(func(_ int, selection *goquery.Selection) {
		if content, ok := selection.Attr("content"); ok {
			setChannelID(channelIDFromYouTubePageURL(content))
		}
	})
	doc.Find("script").Each(func(_ int, selection *goquery.Selection) {
		if channelID != "" {
			return
		}
		scriptText := selection.Text()
		for _, pattern := range []string{`"channelId":"`, `"channel_id":"`, `"externalChannelId":"`} {
			start := strings.Index(scriptText, pattern)
			if start < 0 {
				continue
			}
			start += len(pattern)
			end := strings.Index(scriptText[start:], `"`)
			if end >= 0 {
				setChannelID(scriptText[start : start+end])
			}
		}
	})

	if channelID == "" {
		return "", fmt.Errorf("YouTube response did not contain a valid channel ID")
	}
	return channelID, nil
}

func channelIDFromYouTubePageURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, segment := range segments {
		if segment == "channel" && index+1 < len(segments) {
			return normalizeYouTubeChannelID(segments[index+1])
		}
	}
	return ""
}

// GetChannelVideos returns videos for either a valid channel ID or a valid handle.
func (a *App) GetChannelVideos(channel string) ([]YouTubeVideo, error) {
	return a.getChannelVideos(a.requestContext(), channel, true)
}

func (a *App) getChannelVideos(ctx context.Context, channel string, useCache bool) ([]YouTubeVideo, error) {
	channelID, _, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return nil, err
	}

	if useCache {
		if videos, ok := youtubeCache.getVideos(channelID); ok {
			return videos, nil
		}
	}

	videos, err := a.fetchYouTubeVideos(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YouTube videos: %w", err)
	}

	if useCache {
		youtubeCache.setVideos(channelID, videos)
	}

	return videos, nil
}

// GetChannelVideosPaginated returns the latest videos. The YouTube RSS feed
// does not support a pagination cursor.
func (a *App) GetChannelVideosPaginated(channelID string, before int) ([]YouTubeVideo, error) {
	return a.GetChannelVideos(channelID)
}

func (a *App) fetchYouTubeVideos(ctx context.Context, channelID string) ([]YouTubeVideo, error) {
	channelID = normalizeYouTubeChannelID(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("invalid YouTube channel ID")
	}

	body, _, err := a.httpClient.get(
		ctx,
		providerYouTube,
		youtubeFeedURL(channelID),
		http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			"Accept":     []string{"application/xml, text/xml, */*;q=0.8"},
		},
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}

	videos, err := parseYouTubeFeed(body, channelID)
	if err != nil {
		return nil, fmt.Errorf("YouTube response could not be parsed: %w", err)
	}
	return videos, nil
}

func youtubeFeedURL(channelID string) *url.URL {
	endpoint := (&url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
	}).JoinPath("feeds", "videos.xml")
	query := endpoint.Query()
	query.Set("channel_id", channelID)
	endpoint.RawQuery = query.Encode()
	return endpoint
}

func youtubeWatchURL(videoID string) *url.URL {
	endpoint := &url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "/watch",
	}
	query := endpoint.Query()
	query.Set("v", videoID)
	endpoint.RawQuery = query.Encode()
	return endpoint
}

func parseYouTubeFeed(body []byte, channelID string) ([]YouTubeVideo, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	videos := make([]YouTubeVideo, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		videoID := normalizeYouTubeVideoID(entry.VideoID)
		if videoID == "" {
			continue
		}

		videos = append(videos, YouTubeVideo{
			VideoID:      videoID,
			Title:        strings.TrimSpace(entry.Title),
			PublishedAt:  entry.Published,
			ChannelID:    channelID,
			ChannelTitle: strings.TrimSpace(entry.Author.Name),
			VideoURL:     youtubeWatchURL(videoID).String(),
			Thumbnail:    strings.TrimSpace(entry.MediaGroup.Thumbnail.URL),
		})
	}

	return videos, nil
}

// Atom feed structures for RSS parsing.
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []atomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

type atomEntry struct {
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

func (a *App) resolveYouTubeChannelReference(ctx context.Context, value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("YouTube channel ID or handle is required")
	}
	if channelID := normalizeYouTubeChannelID(value); channelID != "" {
		return channelID, "", nil
	}

	handle := normalizeYouTubeUsername(value)
	if handle == "" {
		return "", "", fmt.Errorf("invalid YouTube channel ID or handle")
	}
	channelID, err := a.resolveChannelID(ctx, handle)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve YouTube handle: %w", err)
	}
	return channelID, handle, nil
}

// AddYouTubeFavorite adds a valid channel ID or resolves a valid handle before storing it.
func (a *App) AddYouTubeFavorite(channel string) ([]string, error) {
	if strings.TrimSpace(channel) == "" {
		return a.ListYouTubeFavorites()
	}

	channelID, handle, err := a.resolveYouTubeChannelReference(a.requestContext(), channel)
	if err != nil {
		return nil, err
	}

	_, err = a.db.Exec(
		`INSERT INTO youtube_favorites (channel_id, username)
		VALUES (?, NULLIF(?, ''))
		ON CONFLICT(channel_id) DO UPDATE SET
			username = COALESCE(NULLIF(excluded.username, ''), youtube_favorites.username)`,
		channelID, handle,
	)
	if err != nil {
		return nil, err
	}

	return a.ListYouTubeFavorites()
}

// RemoveYouTubeFavorite removes a valid channel ID or resolved handle from favorites.
func (a *App) RemoveYouTubeFavorite(channel string) ([]string, error) {
	if strings.TrimSpace(channel) == "" {
		return a.ListYouTubeFavorites()
	}

	channelID, _, err := a.resolveYouTubeChannelReference(a.requestContext(), channel)
	if err != nil {
		return nil, err
	}

	_, err = a.db.Exec(`DELETE FROM youtube_favorites WHERE channel_id = ?`, channelID)
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
		if err := rows.Scan(&id); err != nil {
			return []string{}, err
		}
		favorites = append(favorites, id)
	}
	if err := rows.Err(); err != nil {
		return []string{}, err
	}
	return favorites, nil
}

func (a *App) AssignYouTubeFavoriteCategory(channel string, categoryID int) error {
	channelID, _, err := a.resolveYouTubeChannelReference(a.requestContext(), channel)
	if err != nil {
		return err
	}
	if categoryID <= 0 {
		return fmt.Errorf("invalid category ID")
	}
	if err := a.ensureFavoriteCategoryExists(categoryID, favoriteSourceYouTube); err != nil {
		return err
	}

	_, err = a.db.Exec(
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
