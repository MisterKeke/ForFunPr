package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Task is the public task representation returned by the HTTP API.
type Task struct {
	ID          int           `json:"id"`
	DueDate     string        `json:"due_date"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Priority    string        `json:"priority"`
	Done        bool          `json:"done"`
	CreatedAt   string        `json:"created_at"`
	Difficulty  string        `json:"difficulty"`
	Tags        []string      `json:"tags"`
	Subtasks    []TaskSubtask `json:"subtasks"`
}

type TaskSubtask struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int    `json:"position"`
}

type TaskSubtaskInput struct {
	ID       int    `json:"id,omitempty"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int    `json:"position,omitempty"`
}

type TaskWriteRequest struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Priority    string              `json:"priority"`
	DueDate     string              `json:"due_date"`
	Difficulty  *string             `json:"difficulty,omitempty"`
	Tags        *[]string           `json:"tags,omitempty"`
	Subtasks    *[]TaskSubtaskInput `json:"subtasks,omitempty"`
}

type TaskListFilter struct {
	Query      string
	Date       string
	Priority   string
	Difficulty string
	Tags       []string
}

// ListTasks delegates filtering and validation to GET /api/v1/tasks.
// Empty filter fields do not restrict the task list.
func (c *Client) ListTasks(ctx context.Context, filter TaskListFilter) ([]Task, error) {
	query := make(url.Values)
	if filter.Query != "" {
		query.Set("q", filter.Query)
	}
	if filter.Date != "" {
		query.Set("date", filter.Date)
	}
	if filter.Priority != "" {
		query.Set("priority", filter.Priority)
	}
	if filter.Difficulty != "" {
		query.Set("difficulty", filter.Difficulty)
	}
	for _, tag := range filter.Tags {
		query.Add("tag", tag)
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

func (c *Client) ToggleTaskSubtask(
	ctx context.Context,
	todoID int,
	subtaskID int,
) ([]Task, error) {
	var tasks []Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/subtasks/%d/toggle", todoID, subtaskID),
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
