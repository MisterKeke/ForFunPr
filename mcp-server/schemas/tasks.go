package schemas

// TaskListInput optionally searches and filters tasks.
type TaskListInput struct {
	Query      string   `json:"query,omitempty"`
	Date       string   `json:"date,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	Difficulty string   `json:"difficulty,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	DueFrom    string   `json:"due_from,omitempty"`
	DueTo      string   `json:"due_to,omitempty"`
	Completion string   `json:"completion,omitempty"`
	Overdue    bool     `json:"overdue,omitempty"`
	Undated    bool     `json:"undated,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Direction  string   `json:"direction,omitempty"`
	Limit      *int     `json:"limit,omitempty"`
	Offset     *int     `json:"offset,omitempty"`
}

// TaskListInputSchema is the explicit MCP schema for listing tasks.
var TaskListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "Optional text found in a task title, description, tag, or subtask.",
			"maxLength":   256,
		},
		"date": map[string]any{
			"type":        "string",
			"description": "Optional due date in YYYY-MM-DD format.",
			"pattern":     `^\d{4}-\d{2}-\d{2}$`,
		},
		"priority": map[string]any{
			"type":        "string",
			"description": "Optional exact task priority.",
			"enum":        []string{"low", "medium", "high"},
		},
		"difficulty": map[string]any{
			"type":        "string",
			"description": "Optional exact task difficulty; unset selects tasks without one.",
			"enum":        []string{"unset", "easy", "medium", "hard"},
		},
		"tags": map[string]any{
			"type":        "array",
			"description": "Exact task tags. A task must contain every supplied tag.",
			"maxItems":    32,
			"items":       map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
		},
		"due_from":   map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		"due_to":     map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		"completion": map[string]any{"type": "string", "enum": []string{"all", "complete", "incomplete"}},
		"overdue":    map[string]any{"type": "boolean"},
		"undated":    map[string]any{"type": "boolean"},
		"sort":       map[string]any{"type": "string", "enum": []string{"created_at", "updated_at", "due_date", "priority", "title", "id"}},
		"direction":  map[string]any{"type": "string", "enum": []string{"asc", "desc"}},
		"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
		"offset":     map[string]any{"type": "integer", "minimum": 0},
	},
	"additionalProperties": false,
}

// TaskIDInput selects one task by positive integer ID.
type TaskIDInput struct {
	ID               int  `json:"id"`
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

type TaskSubtaskIDInput struct {
	TaskID           int  `json:"task_id"`
	SubtaskID        int  `json:"subtask_id"`
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

var TaskSubtaskIDInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"task_id": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "Positive parent task ID.",
		},
		"subtask_id": map[string]any{
			"type": "integer", "minimum": 1,
			"description": "Positive subtask ID.",
		},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	},
	"required":             []string{"task_id", "subtask_id"},
	"additionalProperties": false,
}

// TaskIDInputSchema is the explicit MCP schema for one-task mutations.
var TaskIDInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{
			"type":        "integer",
			"description": "Positive task ID.",
			"minimum":     1,
		},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	},
	"required":             []string{"id"},
	"additionalProperties": false,
}

// CreateTaskInput contains the flags accepted by `something tasks create`.
type CreateTaskInput struct {
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	Priority    string             `json:"priority,omitempty"`
	DueDate     string             `json:"due_date,omitempty"`
	Difficulty  string             `json:"difficulty,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Subtasks    []TaskSubtaskInput `json:"subtasks,omitempty"`
}

// CreateTaskInputSchema is the explicit MCP schema for creating a task.
var CreateTaskInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title": map[string]any{
			"type":        "string",
			"description": "Required non-empty task title.",
			"minLength":   1,
		},
		"description": map[string]any{
			"type":        "string",
			"description": "Optional task description.",
		},
		"priority": map[string]any{
			"type":        "string",
			"description": "Optional task priority. Defaults to medium.",
			"enum":        []string{"low", "medium", "high"},
		},
		"due_date": map[string]any{
			"type":        "string",
			"description": "Optional due date in YYYY-MM-DD format.",
			"pattern":     `^\d{4}-\d{2}-\d{2}$`,
		},
		"difficulty": map[string]any{
			"type": "string", "enum": []string{"easy", "medium", "hard"},
			"description": "Optional task difficulty.",
		},
		"tags": map[string]any{
			"type": "array", "maxItems": 32,
			"items":       map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
			"description": "Optional task tags.",
		},
		"subtasks": map[string]any{
			"type": "array", "maxItems": 100,
			"items":       TaskSubtaskInputSchema,
			"description": "Initial subtasks; IDs must be omitted and difficulty must be hard.",
		},
	},
	"required":             []string{"title"},
	"additionalProperties": false,
}

// UpdateTaskInput contains the flags accepted by `something tasks update`.
type UpdateTaskInput struct {
	ID               int                `json:"id"`
	Title            string             `json:"title"`
	Description      string             `json:"description,omitempty"`
	Priority         string             `json:"priority,omitempty"`
	DueDate          string             `json:"due_date,omitempty"`
	Difficulty       string             `json:"difficulty,omitempty"`
	Tags             []string           `json:"tags,omitempty"`
	Subtasks         []TaskSubtaskInput `json:"subtasks,omitempty"`
	ClearDifficulty  bool               `json:"clear_difficulty,omitempty"`
	ClearTags        bool               `json:"clear_tags,omitempty"`
	ClearSubtasks    bool               `json:"clear_subtasks,omitempty"`
	ExpectedRevision *int               `json:"expected_revision,omitempty"`
}

// UpdateTaskInputSchema is the explicit MCP schema for overwriting a task.
var UpdateTaskInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{
			"type":        "integer",
			"description": "Positive task ID.",
			"minimum":     1,
		},
		"title": map[string]any{
			"type":        "string",
			"description": "Required replacement task title.",
			"minLength":   1,
		},
		"description": map[string]any{
			"type":        "string",
			"description": "Replacement description. Omitted means empty.",
		},
		"priority": map[string]any{
			"type":        "string",
			"description": "Replacement priority. Omitted defaults to medium.",
			"enum":        []string{"low", "medium", "high"},
		},
		"due_date": map[string]any{
			"type":        "string",
			"description": "Replacement due date. Omitted clears it.",
			"pattern":     `^\d{4}-\d{2}-\d{2}$`,
		},
		"difficulty": map[string]any{
			"type": "string", "enum": []string{"easy", "medium", "hard"},
			"description": "Replacement difficulty when supplied.",
		},
		"tags": map[string]any{
			"type": "array", "maxItems": 32,
			"items":       map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
			"description": "Replacement tag set when supplied.",
		},
		"subtasks": map[string]any{
			"type": "array", "maxItems": 100,
			"items":       TaskSubtaskInputSchema,
			"description": "Replacement subtasks; use existing IDs to preserve identity.",
		},
		"clear_difficulty":  map[string]any{"type": "boolean"},
		"clear_tags":        map[string]any{"type": "boolean"},
		"clear_subtasks":    map[string]any{"type": "boolean"},
		"expected_revision": map[string]any{"type": "integer", "minimum": 1},
	},
	"required":             []string{"id", "title"},
	"additionalProperties": false,
}

// Task is the JSON item emitted by task CLI commands.
type Task struct {
	ID          int           `json:"id" jsonschema:"Positive task ID."`
	DueDate     string        `json:"due_date" jsonschema:"Due date, or an empty string when none is set."`
	Title       string        `json:"title" jsonschema:"Task title."`
	Description string        `json:"description" jsonschema:"Task description."`
	Priority    string        `json:"priority" jsonschema:"Task priority."`
	Done        bool          `json:"done" jsonschema:"Whether the task is complete."`
	CreatedAt   string        `json:"created_at" jsonschema:"Task creation timestamp."`
	UpdatedAt   string        `json:"updated_at" jsonschema:"Task update timestamp."`
	Revision    int           `json:"revision" jsonschema:"Optimistic concurrency revision."`
	DueState    string        `json:"due_state" jsonschema:"overdue, due_today, upcoming, or unscheduled."`
	Difficulty  string        `json:"difficulty" jsonschema:"Task difficulty, or an empty string when unset."`
	Tags        []string      `json:"tags" jsonschema:"Task tags; empty for tasks without tags."`
	Subtasks    []TaskSubtask `json:"subtasks" jsonschema:"Ordered hard-task subtasks."`
}

type TaskSubtask struct {
	ID       int    `json:"id" jsonschema:"Positive subtask ID."`
	Title    string `json:"title" jsonschema:"Subtask title."`
	Done     bool   `json:"done" jsonschema:"Whether the subtask is complete."`
	Position int    `json:"position" jsonschema:"Zero-based display position."`
}

type TaskSubtaskInput struct {
	ID       int    `json:"id,omitempty"`
	Title    string `json:"title"`
	Done     bool   `json:"done,omitempty"`
	Position int    `json:"position,omitempty"`
}

var TaskSubtaskInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":       map[string]any{"type": "integer", "minimum": 0},
		"title":    map[string]any{"type": "string", "minLength": 1},
		"done":     map[string]any{"type": "boolean"},
		"position": map[string]any{"type": "integer", "minimum": 0},
	},
	"required":             []string{"title"},
	"additionalProperties": false,
}

// TasksOutput wraps the top-level JSON array emitted by task CLI commands so
// MCP structuredContent remains a JSON object.
type TasksOutput struct {
	Tasks []Task `json:"tasks" jsonschema:"Exact task items emitted by the CLI command."`
}

type TaskListOutput struct {
	Items     []Task `json:"items"`
	Total     int    `json:"total"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	HasMore   bool   `json:"has_more"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type TaskOutput struct {
	Task Task `json:"task"`
}

type TaskDeletionOutput struct {
	DeletedID       int `json:"deleted_id"`
	DeletedRevision int `json:"deleted_revision"`
}

type TaskDateInput struct {
	IncludeOverdue bool `json:"include_overdue,omitempty"`
	IncludeUndated bool `json:"include_undated,omitempty"`
	WeekStart      *int `json:"week_start,omitempty"`
}

var TaskDateInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"include_overdue": map[string]any{"type": "boolean"},
		"include_undated": map[string]any{"type": "boolean"},
		"week_start":      map[string]any{"type": "integer", "minimum": 0, "maximum": 6},
	},
	"additionalProperties": false,
}

type TodayTasksOutput struct {
	Overdue     []Task `json:"overdue"`
	DueToday    []Task `json:"due_today"`
	Unscheduled []Task `json:"unscheduled"`
	Date        string `json:"date"`
	TimeZone    string `json:"time_zone"`
}

type WeekTasksOutput struct {
	Items       []Task `json:"items"`
	Overdue     []Task `json:"overdue"`
	Unscheduled []Task `json:"unscheduled"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	WeekStart   int    `json:"week_start"`
	TimeZone    string `json:"time_zone"`
}
