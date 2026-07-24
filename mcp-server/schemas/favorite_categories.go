package schemas

// FavoriteCategoryListInput optionally selects the source. The CLI and API
// default to Telegram when it is omitted.
type FavoriteCategoryListInput struct {
	Source string `json:"source,omitempty"`
}

// FavoriteCategoryListInputSchema is the explicit MCP schema for listing
// source-scoped favorite categories.
var FavoriteCategoryListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"source": map[string]any{
			"type":        "string",
			"description": "Optional source. Defaults to telegram.",
			"enum":        []string{"telegram", "youtube"},
		},
	},
	"additionalProperties": false,
}

// CreateFavoriteCategoryInput creates or returns a source-scoped category.
type CreateFavoriteCategoryInput struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// CreateFavoriteCategoryInputSchema is the explicit MCP schema for creating a
// favorite category.
var CreateFavoriteCategoryInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Required non-empty category name.",
			"minLength":   1,
		},
		"source": map[string]any{
			"type":        "string",
			"description": "Category source.",
			"enum":        []string{"telegram", "youtube"},
		},
	},
	"required":             []string{"name", "source"},
	"additionalProperties": false,
}

// FavoriteCategory is the complete category object emitted by the CLI.
type FavoriteCategory struct {
	ID        int    `json:"id" jsonschema:"Positive category ID."`
	Name      string `json:"name" jsonschema:"Category display name."`
	Source    string `json:"source" jsonschema:"Category source."`
	Color     string `json:"color,omitempty" jsonschema:"Optional category color."`
	CreatedAt string `json:"created_at" jsonschema:"Category creation timestamp."`
}

// FavoriteCategoriesOutput is the JSON emitted by the category list command.
type FavoriteCategoriesOutput struct {
	Categories []FavoriteCategory `json:"categories" jsonschema:"Favorite categories for the selected source."`
}
