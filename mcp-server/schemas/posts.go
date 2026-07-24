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

// PostsOutput wraps the top-level JSON array emitted by a posts CLI command so
// MCP structuredContent remains a JSON object.
type PostsOutput struct {
	Posts []ChannelDate `json:"posts" jsonschema:"Exact projected items emitted by the posts CLI command."`
}
