package schemas

type Note struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NoteSummary struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NotesOutput struct {
	Notes  []NoteSummary `json:"notes"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type NoteListInput struct {
	Query   string `json:"query,omitempty"`
	Archive string `json:"archive,omitempty"`
	Pinned  *bool  `json:"pinned,omitempty"`
	Limit   *int   `json:"limit,omitempty"`
	Offset  *int   `json:"offset,omitempty"`
}

var NoteListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"query":   map[string]any{"type": "string", "maxLength": 256},
		"archive": map[string]any{"type": "string", "enum": []string{"active", "archived", "all"}},
		"pinned":  map[string]any{"type": "boolean"},
		"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
		"offset":  map[string]any{"type": "integer", "minimum": 0},
	},
	"additionalProperties": false,
}

type NoteIDInput struct {
	ID int `json:"id"`
}

var NoteIDInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "integer", "minimum": 1},
	},
	"required":             []string{"id"},
	"additionalProperties": false,
}

type CreateNoteInput struct {
	Title  string `json:"title,omitempty"`
	Body   string `json:"body,omitempty"`
	Pinned bool   `json:"pinned,omitempty"`
}

var CreateNoteInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title":  map[string]any{"type": "string", "maxLength": 200},
		"body":   map[string]any{"type": "string", "maxLength": 262144},
		"pinned": map[string]any{"type": "boolean"},
	},
	"anyOf": []any{
		map[string]any{"required": []string{"title"}},
		map[string]any{"required": []string{"body"}},
	},
	"additionalProperties": false,
}

type UpdateNoteInput struct {
	ID    int     `json:"id"`
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
}

var UpdateNoteInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":    map[string]any{"type": "integer", "minimum": 1},
		"title": map[string]any{"type": "string", "maxLength": 200},
		"body":  map[string]any{"type": "string", "maxLength": 262144},
	},
	"required": []string{"id"},
	"anyOf": []any{
		map[string]any{"required": []string{"title"}},
		map[string]any{"required": []string{"body"}},
	},
	"additionalProperties": false,
}

type NoteStateInput struct {
	ID    int  `json:"id"`
	Value bool `json:"value"`
}

var NoteStateInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":    map[string]any{"type": "integer", "minimum": 1},
		"value": map[string]any{"type": "boolean"},
	},
	"required":             []string{"id", "value"},
	"additionalProperties": false,
}
