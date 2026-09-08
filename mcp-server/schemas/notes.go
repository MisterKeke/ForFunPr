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

type NoteTopic struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	BlockCount int    `json:"block_count"`
	Revision   int    `json:"revision"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type NoteTopicBlock struct {
	ID                 int     `json:"id"`
	TopicID            int     `json:"topic_id"`
	NoteID             int     `json:"note_id"`
	PositionX          float64 `json:"position_x"`
	PositionY          float64 `json:"position_y"`
	Title              string  `json:"title"`
	Preview            string  `json:"preview"`
	Pinned             bool    `json:"pinned"`
	Archived           bool    `json:"archived"`
	UpdatedAt          string  `json:"updated_at"`
	LinkedTaskCount    int     `json:"linked_task_count"`
	CompletedTaskCount int     `json:"completed_task_count"`
}

type NoteTopicConnection struct {
	ID           int    `json:"id"`
	TopicID      int    `json:"topic_id"`
	FromBlockID  int    `json:"from_block_id"`
	ToBlockID    int    `json:"to_block_id"`
	RelationType string `json:"relation_type"`
	CreatedAt    string `json:"created_at"`
}

type NoteTopicBoard struct {
	Topic       NoteTopic             `json:"topic"`
	Blocks      []NoteTopicBlock      `json:"blocks"`
	Connections []NoteTopicConnection `json:"connections"`
}

type NoteTopicsOutput struct {
	Items  []NoteTopic `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type NoteTopicPickerOutput struct {
	Items  []NoteSummary `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type NoteTopicMutationOutput struct {
	Changed       bool `json:"changed"`
	TopicID       int  `json:"topic_id,omitempty"`
	TopicRevision int  `json:"topic_revision,omitempty"`
}

type NoteTodoMutationOutput struct {
	Changed bool `json:"changed"`
	NoteID  int  `json:"note_id"`
	TodoID  int  `json:"todo_id"`
}

type NoteTasksOutput struct {
	Items []Task `json:"items"`
}

type TaskNotesOutput struct {
	Items []NoteSummary `json:"items"`
}

type TaskNotesInput struct {
	TaskID int `json:"task_id"`
}

var TaskNotesInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"task_id": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"task_id"}, "additionalProperties": false,
}

type NoteTopicListInput struct {
	Limit  *int `json:"limit,omitempty"`
	Offset *int `json:"offset,omitempty"`
}

var NoteTopicListInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
		"offset": map[string]any{"type": "integer", "minimum": 0},
	}, "additionalProperties": false,
}

type NoteTopicIDInput struct {
	ID               int  `json:"id"`
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

var NoteTopicIDInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"id":                map[string]any{"type": "integer", "minimum": 1},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"id"}, "additionalProperties": false,
}

type NoteTopicPickerInput struct {
	ID     int    `json:"id"`
	Query  string `json:"query,omitempty"`
	Limit  *int   `json:"limit,omitempty"`
	Offset *int   `json:"offset,omitempty"`
}

var NoteTopicPickerInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"id":     map[string]any{"type": "integer", "minimum": 1},
		"query":  map[string]any{"type": "string", "maxLength": 256},
		"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
		"offset": map[string]any{"type": "integer", "minimum": 0},
	}, "required": []string{"id"}, "additionalProperties": false,
}

type NoteTopicWriteInput struct {
	ID               int    `json:"id,omitempty"`
	Title            string `json:"title"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

var NoteTopicWriteInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
	}, "required": []string{"title"}, "additionalProperties": false,
}

var UpdateNoteTopicInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"id":                map[string]any{"type": "integer", "minimum": 1},
		"title":             map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"id", "title"}, "additionalProperties": false,
}

type NoteTopicBlockInput struct {
	TopicID          int     `json:"topic_id"`
	NoteID           int     `json:"note_id"`
	PositionX        float64 `json:"position_x,omitempty"`
	PositionY        float64 `json:"position_y,omitempty"`
	ExpectedRevision *int    `json:"expected_revision,omitempty"`
}

var NoteTopicBlockInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"topic_id":          map[string]any{"type": "integer", "minimum": 1},
		"note_id":           map[string]any{"type": "integer", "minimum": 1},
		"position_x":        map[string]any{"type": "number", "minimum": 0, "maximum": 100000},
		"position_y":        map[string]any{"type": "number", "minimum": 0, "maximum": 100000},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"topic_id", "note_id"}, "additionalProperties": false,
}

type NoteTopicPosition struct {
	BlockID   int     `json:"block_id"`
	PositionX float64 `json:"position_x"`
	PositionY float64 `json:"position_y"`
}

type NoteTopicPositionsInput struct {
	TopicID          int                 `json:"topic_id"`
	Positions        []NoteTopicPosition `json:"positions"`
	ExpectedRevision *int                `json:"expected_revision,omitempty"`
}

var NoteTopicPositionsInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"topic_id": map[string]any{"type": "integer", "minimum": 1},
		"positions": map[string]any{"type": "array", "minItems": 1, "maxItems": 200, "items": map[string]any{
			"type": "object", "properties": map[string]any{
				"block_id":   map[string]any{"type": "integer", "minimum": 1},
				"position_x": map[string]any{"type": "number", "minimum": 0, "maximum": 100000},
				"position_y": map[string]any{"type": "number", "minimum": 0, "maximum": 100000},
			}, "required": []string{"block_id", "position_x", "position_y"}, "additionalProperties": false,
		}},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"topic_id", "positions"}, "additionalProperties": false,
}

type NoteTopicConnectionInput struct {
	TopicID          int    `json:"topic_id"`
	FromBlockID      int    `json:"from_block_id"`
	ToBlockID        int    `json:"to_block_id"`
	RelationType     string `json:"relation_type,omitempty"`
	ExpectedRevision *int   `json:"expected_revision,omitempty"`
}

var NoteTopicConnectionInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"topic_id":          map[string]any{"type": "integer", "minimum": 1},
		"from_block_id":     map[string]any{"type": "integer", "minimum": 1},
		"to_block_id":       map[string]any{"type": "integer", "minimum": 1},
		"relation_type":     map[string]any{"type": "string", "enum": []string{"leads_to", "related"}},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"topic_id", "from_block_id", "to_block_id"}, "additionalProperties": false,
}

type NoteTodoInput struct {
	NoteID int `json:"note_id"`
	TodoID int `json:"todo_id,omitempty"`
}

var NoteTodoInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"note_id": map[string]any{"type": "integer", "minimum": 1},
		"todo_id": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"note_id"}, "additionalProperties": false,
}

var NoteTodoWriteInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"note_id": map[string]any{"type": "integer", "minimum": 1},
		"todo_id": map[string]any{"type": "integer", "minimum": 1},
	}, "required": []string{"note_id", "todo_id"}, "additionalProperties": false,
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
