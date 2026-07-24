package schemas

// TaskListInput optionally filters tasks by due date.
type TaskListInput struct {
	Date string `json:"date,omitempty"`
}

// TaskListInputSchema is the explicit MCP schema for listing tasks.
var TaskListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"date": map[string]any{
			"type":        "string",
			"description": "Optional due date in YYYY-MM-DD format.",
			"pattern":     `^\d{4}-\d{2}-\d{2}$`,
		},
	},
	"additionalProperties": false,
}

// TaskIDInput selects one task by positive integer ID.
type TaskIDInput struct {
	ID int `json:"id"`
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
	},
	"required":             []string{"id"},
	"additionalProperties": false,
}

// CreateTaskInput contains the flags accepted by `something tasks create`.
type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
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
	},
	"required":             []string{"title"},
	"additionalProperties": false,
}

// UpdateTaskInput contains the flags accepted by `something tasks update`.
type UpdateTaskInput struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
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
	},
	"required":             []string{"id", "title"},
	"additionalProperties": false,
}

// Task is the JSON item emitted by task CLI commands.
type Task struct {
	ID          int    `json:"id" jsonschema:"Positive task ID."`
	DueDate     string `json:"due_date" jsonschema:"Due date, or an empty string when none is set."`
	Title       string `json:"title" jsonschema:"Task title."`
	Description string `json:"description" jsonschema:"Task description."`
	Priority    string `json:"priority" jsonschema:"Task priority."`
	Done        bool   `json:"done" jsonschema:"Whether the task is complete."`
	CreatedAt   string `json:"created_at" jsonschema:"Task creation timestamp."`
}

// TasksOutput wraps the top-level JSON array emitted by task CLI commands so
// MCP structuredContent remains a JSON object.
type TasksOutput struct {
	Tasks []Task `json:"tasks" jsonschema:"Exact task items emitted by the CLI command."`
}
