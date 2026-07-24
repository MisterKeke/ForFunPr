package apiclient

import (
	"context"
	"fmt"
	"net/http"
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

type TaskWriteRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

// ListTasks delegates filtering and validation to GET /api/v1/tasks.
// An empty date requests all tasks.
func (c *Client) ListTasks(ctx context.Context, date string) ([]Task, error) {
	query := make(url.Values)
	if date != "" {
		query.Set("date", date)
	}

	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/v1/tasks",
		query,
		nil,
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) CreateTask(
	ctx context.Context,
	request TaskWriteRequest,
) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/v1/tasks",
		nil,
		request,
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) TodayTasks(ctx context.Context) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/v1/tasks/today",
		nil,
		nil,
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) UpdateTask(
	ctx context.Context,
	id int,
	request TaskWriteRequest,
) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/tasks/%d", id),
		nil,
		request,
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) ToggleTask(ctx context.Context, id int) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/toggle", id),
		nil,
		struct{}{},
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) DeleteTask(ctx context.Context, id int) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/tasks/%d", id),
		nil,
		nil,
		&tasks,
	); err != nil {
		return nil, err
	}

	return tasks, nil
}
