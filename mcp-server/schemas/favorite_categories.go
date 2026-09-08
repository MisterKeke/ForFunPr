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
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color,omitempty"`
	DisplayOrder *int   `json:"display_order,omitempty"`
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
		"color":         map[string]any{"type": "string", "pattern": `^$|^#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3})?$`},
		"display_order": map[string]any{"type": "integer", "minimum": 0, "maximum": 1000000},
	},
	"required":             []string{"name", "source"},
	"additionalProperties": false,
}

type RenameFavoriteCategoryInput struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Color        *string `json:"color,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

var RenameFavoriteCategoryInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "Positive category ID.",
		},
		"name": map[string]any{
			"type": "string", "minLength": 1,
			"description": "Required replacement category name.",
		},
		"color":         map[string]any{"type": "string", "pattern": `^$|^#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3})?$`},
		"display_order": map[string]any{"type": "integer", "minimum": 0, "maximum": 1000000},
	},
	"required":             []string{"id", "name"},
	"additionalProperties": false,
}

// FavoriteCategory is the complete category object emitted by the CLI.
type FavoriteCategory struct {
	ID           int    `json:"id" jsonschema:"Positive category ID."`
	Name         string `json:"name" jsonschema:"Category display name."`
	Source       string `json:"source" jsonschema:"Category source."`
	Color        string `json:"color,omitempty" jsonschema:"Optional category color."`
	CreatedAt    string `json:"created_at" jsonschema:"Category creation timestamp."`
	UpdatedAt    string `json:"updated_at" jsonschema:"Category update timestamp."`
	DisplayOrder int    `json:"display_order" jsonschema:"Category display order."`
}

// FavoriteCategoriesOutput is the JSON emitted by the category list command.
type FavoriteCategoriesOutput struct {
	Categories []FavoriteCategory `json:"categories" jsonschema:"Favorite categories for the selected source."`
}

type FavoriteCategoryMutationOutput struct {
	Category          FavoriteCategory `json:"category"`
	Changed           bool             `json:"changed"`
	AffectedFavorites int              `json:"affected_favorites"`
}

type DeleteFavoriteCategoryInput struct {
	ID               int    `json:"id"`
	Mode             string `json:"mode"`
	TargetCategoryID *int   `json:"target_category_id,omitempty"`
}

var DeleteFavoriteCategoryInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":                 map[string]any{"type": "integer", "minimum": 1},
		"mode":               map[string]any{"type": "string", "enum": []string{"unassign", "move"}},
		"target_category_id": map[string]any{"type": "integer", "minimum": 1},
	},
	"required":             []string{"id", "mode"},
	"additionalProperties": false,
}

type FavoriteCategoryDeleteOutput struct {
	DeletedID         int    `json:"deleted_id"`
	Source            string `json:"source"`
	Changed           bool   `json:"changed"`
	AffectedFavorites int    `json:"affected_favorites"`
}

type ReorderFavoriteCategoriesInput struct {
	Source      string `json:"source"`
	CategoryIDs []int  `json:"category_ids"`
}

var ReorderFavoriteCategoriesInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"source":       map[string]any{"type": "string", "enum": []string{"telegram", "youtube"}},
		"category_ids": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "integer", "minimum": 1}},
	},
	"required":             []string{"source", "category_ids"},
	"additionalProperties": false,
}
