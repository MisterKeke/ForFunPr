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

type youtubeVideoDetailsResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Description string `json:"description"`
		} `json:"snippet"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Statistics struct {
			ViewCount string `json:"viewCount"`
		} `json:"statistics"`
	} `json:"items"`
}

// resolveChannelID fetches and parses the public channel page for one valid
// YouTube handle. The result is always a validated UC... channel ID.
func (a *Service) resolveChannelID(ctx context.Context, handle string) (string, error) {
	handle = normalizeYouTubeUsername(handle)
	if handle == "" {
		return "", fmt.Errorf("invalid YouTube handle")
	}

	cacheKey := strings.ToLower(handle)
	if channelID, ok := a.youTubeHandles.get(cacheKey); ok {
		return channelID, nil
	}

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

	a.youTubeHandles.set(cacheKey, channelID)

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
func (a *Service) GetChannelVideos(channel string) ([]YouTubeVideo, error) {
	return a.GetChannelVideosContext(a.requestContext(), channel)
}

func (a *Service) GetChannelVideosContext(ctx context.Context, channel string) ([]YouTubeVideo, error) {
	return a.getChannelVideos(ctx, channel, true)
}

func (a *Service) getChannelVideos(ctx context.Context, channel string, useCache bool) ([]YouTubeVideo, error) {
	channelID, _, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return nil, err
	}

	if useCache {
		if videos, ok := a.youTubeVideos.get(channelID); ok {
			return videos, nil
		}
	}

	videos, err := a.fetchYouTubeVideos(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YouTube videos: %w", err)
	}

	if useCache {
		if len(videos) > 0 {
			a.youTubeVideos.set(channelID, videos)
		}
	}

	return videos, nil
}

// GetChannelVideosPaginated returns the latest videos. The YouTube RSS feed
// does not support a pagination cursor.
func (a *Service) GetChannelVideosPaginated(channelID string, before int) ([]YouTubeVideo, error) {
	return a.GetChannelVideosPaginatedContext(a.requestContext(), channelID, before)
}

func (a *Service) GetChannelVideosPaginatedContext(ctx context.Context, channelID string, before int) ([]YouTubeVideo, error) {
	return nil, &UnsupportedPaginationError{Source: favoriteSourceYouTube}
}

func (a *Service) RefreshChannelVideosContext(ctx context.Context, channel string) ([]YouTubeVideo, error) {
	channelID, _, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return nil, err
	}
	a.youTubeVideos.invalidate(channelID)
	return a.getChannelVideos(ctx, channelID, true)
}

func (a *Service) fetchYouTubeVideos(ctx context.Context, channelID string) ([]YouTubeVideo, error) {
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
			Description:  strings.TrimSpace(entry.MediaGroup.Description),
			Views:        strings.TrimSpace(entry.MediaGroup.Community.Statistics.Views),
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

		Description string `xml:"http://search.yahoo.com/mrss/ description"`

		Community struct {
			Statistics struct {
				Views string `xml:"views,attr"`
			} `xml:"http://search.yahoo.com/mrss/ statistics"`
		} `xml:"http://search.yahoo.com/mrss/ community"`
	} `xml:"http://search.yahoo.com/mrss/ group"`
	VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
}

func (a *Service) resolveYouTubeChannelReference(ctx context.Context, value string) (string, string, error) {
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
func (a *Service) AddYouTubeFavorite(channel string) ([]string, error) {
	return a.AddYouTubeFavoriteContext(a.requestContext(), channel)
}

func (a *Service) AddYouTubeFavoriteContext(ctx context.Context, channel string) ([]string, error) {
	if strings.TrimSpace(channel) == "" {
		return a.ListYouTubeFavorites()
	}

	channelID, handle, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return nil, err
	}

	result, err := a.db.ExecContext(ctx,
		`INSERT INTO youtube_favorites (channel_id, username)
		VALUES (?, NULLIF(?, ''))
		ON CONFLICT(channel_id) DO UPDATE SET
			username = COALESCE(NULLIF(excluded.username, ''), youtube_favorites.username)`,
		channelID, handle,
	)
	if err != nil {
		return nil, fmt.Errorf("add YouTube favorite: %w", err)
	}
	if err := requireSingleMutation(result, "add YouTube favorite", "YouTube favorite", false); err != nil { return nil, err }

	return a.ListYouTubeFavorites()
}

// RemoveYouTubeFavorite removes a valid channel ID or resolved handle from favorites.
func (a *Service) RemoveYouTubeFavorite(channel string) ([]string, error) {
	return a.RemoveYouTubeFavoriteContext(a.requestContext(), channel)
}

func (a *Service) RemoveYouTubeFavoriteContext(ctx context.Context, channel string) ([]string, error) {
	if strings.TrimSpace(channel) == "" {
		return a.ListYouTubeFavorites()
	}

	channelID, _, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return nil, err
	}

	result, err := a.db.ExecContext(ctx, `DELETE FROM youtube_favorites WHERE channel_id = ?`, channelID)
	if err != nil {
		return nil, fmt.Errorf("remove YouTube favorite: %w", err)
	}
	// Deleting an absent favorite remains intentionally idempotent.
	if err := requireSingleMutation(result, "remove YouTube favorite", "YouTube favorite", true); err != nil { return nil, err }

	return a.ListYouTubeFavorites()
}

// ListYouTubeFavorites returns the list of favorite channel IDs.
func (a *Service) ListYouTubeFavorites() ([]string, error) {
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

func (a *Service) AssignYouTubeFavoriteCategory(channel string, categoryID int) error {
	return a.AssignYouTubeFavoriteCategoryContext(a.requestContext(), channel, categoryID)
}

func (a *Service) AssignYouTubeFavoriteCategoryContext(ctx context.Context, channel string, categoryID int) error {
	channelID, _, err := a.resolveYouTubeChannelReference(ctx, channel)
	if err != nil {
		return err
	}
	if categoryID <= 0 {
		return fmt.Errorf("invalid category ID")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin YouTube category assignment: %w", err) }
	defer tx.Rollback()
	if err := ensureFavoriteCategoryExists(ctx, tx, categoryID, favoriteSourceYouTube); err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE youtube_favorites SET category_id = ? WHERE channel_id = ?`,
		categoryID, channelID,
	)
	if err != nil {
		return fmt.Errorf("failed to assign category: %w", err)
	}
	if err := requireSingleMutation(result, "assign YouTube favorite category", "YouTube favorite "+channelID, false); err != nil { return err }
	return tx.Commit()
}

func (a *Service) ListYouTubeFavoritesWithCategories() ([]FavoriteChannel, error) {
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
