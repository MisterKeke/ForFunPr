package apiclient

import (
	"context"
	"net/url"
)

// Task is the public task representation returned by the HTTP API.
type Task struct {
	ID          int    `json:"id"`
	DueDate     string `json:"due_date"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Done        bool   `json:"done"`
	CreatedAt   string `json:"created_at"`
}

// ListTasks delegates filtering and validation to GET /api/v1/tasks.
// An empty date requests all tasks.
func (c *Client) ListTasks(ctx context.Context, date string) ([]Task, error) {
	query := make(url.Values)
	if date != "" {
		query.Set("date", date)
	}

	var tasks []Task
	if err := c.getJSON(ctx, "/api/v1/tasks", query, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}
