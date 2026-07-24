package schemas

// ChannelInput identifies a favorite channel.
type ChannelInput struct {
	Channel string `json:"channel"`
}

// ChannelInputSchema is the explicit MCP schema for one-channel favorite
// mutations.
var ChannelInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"channel": map[string]any{
			"type":        "string",
			"description": "Required channel username, handle, or ID.",
			"minLength":   1,
		},
	},
	"required":             []string{"channel"},
	"additionalProperties": false,
}

// ChannelCategoryInput assigns a positive category ID to a channel.
type ChannelCategoryInput struct {
	Channel    string `json:"channel"`
	CategoryID int    `json:"category_id"`
}

// ChannelCategoryInputSchema is the explicit MCP schema for assigning a
// favorite category.
var ChannelCategoryInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"channel": map[string]any{
			"type":        "string",
			"description": "Required channel username, handle, or ID.",
			"minLength":   1,
		},
		"category_id": map[string]any{
			"type":        "integer",
			"description": "Positive favorite category ID.",
			"minimum":     1,
		},
	},
	"required":             []string{"channel", "category_id"},
	"additionalProperties": false,
}

// FavoritesOutput is the JSON emitted by favorite list and add commands.
type FavoritesOutput struct {
	Favorites []string `json:"favorites" jsonschema:"Saved channel usernames or IDs."`
}

// CategorizedFavorite is a saved channel with an optional category.
type CategorizedFavorite struct {
	Username   string `json:"username,omitempty" jsonschema:"Telegram username."`
	ChannelID  string `json:"channel_id,omitempty" jsonschema:"YouTube channel ID."`
	CategoryID *int   `json:"category_id,omitempty" jsonschema:"Assigned favorite category ID."`
}

// CategorizedFavoritesOutput is the JSON emitted by list-categories commands.
type CategorizedFavoritesOutput struct {
	Favorites []CategorizedFavorite `json:"favorites" jsonschema:"Saved channels and their category assignments."`
}
