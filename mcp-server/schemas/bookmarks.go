package schemas

type Bookmark struct {
	ID          int      `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Read        bool     `json:"read"`
	ReadAt      string   `json:"read_at"`
	Tags        []string `json:"tags"`
	Revision    int      `json:"revision"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type BookmarksOutput struct {
	Bookmarks []Bookmark `json:"bookmarks"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

type BookmarkTagsOutput struct {
	Tags []string `json:"tags"`
}

type BookmarkListInput struct {
	Query  string   `json:"query,omitempty"`
	Status string   `json:"status,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Limit  *int     `json:"limit,omitempty"`
	Offset *int     `json:"offset,omitempty"`
}

var BookmarkListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"query": map[string]any{"type": "string", "maxLength": 256},
		"status": map[string]any{"type": "string", "enum": []string{"all", "unread", "read"}},
		"tags": map[string]any{
			"type": "array", "maxItems": 32,
			"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		},
		"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
		"offset": map[string]any{"type": "integer", "minimum": 0},
	},
	"additionalProperties": false,
}

type BookmarkIDInput struct {
	ID int `json:"id"`
}

var BookmarkIDInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{"id": map[string]any{"type": "integer", "minimum": 1}},
	"required": []string{"id"},
	"additionalProperties": false,
}

type CreateBookmarkInput struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

var CreateBookmarkInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"url": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096},
		"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
		"description": map[string]any{"type": "string", "maxLength": 16384},
		"tags": map[string]any{
			"type": "array", "maxItems": 32,
			"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		},
	},
	"required": []string{"url", "title"},
	"additionalProperties": false,
}

type UpdateBookmarkInput struct {
	ID          int      `json:"id"`
	URL         *string  `json:"url,omitempty"`
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ClearTags   bool     `json:"clear_tags,omitempty"`
}

var UpdateBookmarkInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "integer", "minimum": 1},
		"url": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096},
		"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
		"description": map[string]any{"type": "string", "maxLength": 16384},
		"tags": map[string]any{
			"type": "array", "maxItems": 32,
			"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		},
		"clear_tags": map[string]any{"type": "boolean"},
	},
	"required": []string{"id"},
	"additionalProperties": false,
}

type BookmarkReadInput struct {
	ID   int  `json:"id"`
	Read bool `json:"read"`
}

var BookmarkReadInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "integer", "minimum": 1},
		"read": map[string]any{"type": "boolean"},
	},
	"required": []string{"id", "read"},
	"additionalProperties": false,
}
