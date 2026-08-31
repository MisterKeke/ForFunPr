package schemas

// ChannelPostsInput selects a channel and optional non-negative cursor.
type ChannelPostsInput struct {
	Channel string `json:"channel"`
	Before  *int   `json:"before,omitempty"`
}

// ChannelPostsInputSchema is the explicit MCP schema for one-channel post
// retrieval tools.
var ChannelPostsInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"channel": map[string]any{
			"type":        "string",
			"description": "Required Telegram username, YouTube handle, or YouTube channel ID.",
			"minLength":   1,
		},
		"before": map[string]any{
			"type":        "integer",
			"description": "Optional non-negative pagination cursor.",
			"minimum":     0,
		},
	},
	"required":             []string{"channel"},
	"additionalProperties": false,
}

// YouTubeChannelPostsInputSchema is cursor-free because the public YouTube
// RSS feed exposes only the latest entries and has no reliable pagination.
var YouTubeChannelPostsInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"channel": map[string]any{
			"type":        "string",
			"description": "Required YouTube handle or YouTube channel ID.",
			"minLength":   1,
		},
	},
	"required":             []string{"channel"},
	"additionalProperties": false,
}

// RefreshChannelPostsInputSchema accepts only the provider reference. Refresh
// operations deliberately have no pagination cursor because they replace the
// provider cache with the latest page.
var RefreshChannelPostsInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"channel": map[string]any{
			"type":        "string",
			"description": "Required Telegram username, YouTube handle, or YouTube channel ID.",
			"minLength":   1,
		},
	},
	"required":             []string{"channel"},
	"additionalProperties": false,
}

// FavoritePostsInput optionally paginates posts from all saved channels.
type FavoritePostsInput struct {
	Before *int `json:"before,omitempty"`
}

// FavoritePostsInputSchema is the explicit MCP schema for favorite-channel
// post retrieval tools.
var FavoritePostsInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"before": map[string]any{
			"type":        "integer",
			"description": "Optional non-negative pagination cursor.",
			"minimum":     0,
		},
	},
	"additionalProperties": false,
}

var YouTubeFavoritePostsInputSchema = map[string]any{
	"type":                 "object",
	"properties":           map[string]any{},
	"additionalProperties": false,
}

// Post is one CLI post with common fields and optional source-specific fields.
type Post struct {
	Source       string   `json:"source" jsonschema:"Post source: telegram or youtube."`
	Date         string   `json:"date" jsonschema:"Publication timestamp returned by the provider."`
	ChannelName  string   `json:"channel_name" jsonschema:"Telegram or YouTube channel name."`
	Views        string   `json:"views,omitempty" jsonschema:"Provider-formatted view count when available."`
	Text         string   `json:"text,omitempty" jsonschema:"Telegram post text."`
	Images       []string `json:"images,omitempty" jsonschema:"Telegram post image URLs."`
	PostID       string   `json:"post_id,omitempty" jsonschema:"Telegram post identifier."`
	PostURL      string   `json:"post_url,omitempty" jsonschema:"Canonical Telegram post URL."`
	VideoID      string   `json:"video_id,omitempty" jsonschema:"YouTube video identifier."`
	Title        string   `json:"title,omitempty" jsonschema:"YouTube video title."`
	Description  string   `json:"description,omitempty" jsonschema:"YouTube video description."`
	Thumbnail    string   `json:"thumbnail,omitempty" jsonschema:"YouTube thumbnail URL."`
	ChannelID    string   `json:"channel_id,omitempty" jsonschema:"YouTube channel identifier."`
	ChannelTitle string   `json:"channel_title,omitempty" jsonschema:"YouTube channel title."`
	VideoURL     string   `json:"video_url,omitempty" jsonschema:"YouTube video URL."`
	Duration     string   `json:"duration,omitempty" jsonschema:"YouTube video duration."`
}

// PostsOutput wraps the top-level JSON array emitted by a posts CLI command so
// MCP structuredContent remains a JSON object.
type PostsOutput struct {
	Posts []Post `json:"posts" jsonschema:"Full posts emitted by the posts CLI command."`
}
