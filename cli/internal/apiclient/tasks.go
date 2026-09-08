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
	UpdatedAt   string        `json:"updated_at"`
	Revision    int           `json:"revision"`
	DueState    string        `json:"due_state"`
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
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	Priority         string              `json:"priority"`
	DueDate          string              `json:"due_date"`
	Difficulty       *string             `json:"difficulty,omitempty"`
	Tags             *[]string           `json:"tags,omitempty"`
	Subtasks         *[]TaskSubtaskInput `json:"subtasks,omitempty"`
	ExpectedRevision *int                `json:"expected_revision,omitempty"`
}

type TaskListFilter struct {
	Query      string
	Date       string
	Priority   string
	Difficulty string
	Tags       []string
	DueFrom    string
	DueTo      string
	Completion string
	Overdue    bool
	Undated    bool
	Sort       string
	Direction  string
	Limit      int
	Offset     int
}

type TaskListResult struct {
	Items     []Task `json:"items"`
	Total     int    `json:"total"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	HasMore   bool   `json:"has_more"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type TaskDateQuery struct {
	IncludeOverdue bool
	IncludeUndated bool
	WeekStart      *int
}

type TodayTaskResult struct {
	Overdue     []Task `json:"overdue"`
	DueToday    []Task `json:"due_today"`
	Unscheduled []Task `json:"unscheduled"`
	Date        string `json:"date"`
	TimeZone    string `json:"time_zone"`
}

type WeekTaskResult struct {
	Items       []Task `json:"items"`
	Overdue     []Task `json:"overdue"`
	Unscheduled []Task `json:"unscheduled"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	WeekStart   int    `json:"week_start"`
	TimeZone    string `json:"time_zone"`
}

type TaskDeletionReceipt struct {
	DeletedID       int `json:"deleted_id"`
	DeletedRevision int `json:"deleted_revision"`
}

// ListTasks delegates filtering and validation to GET /api/v1/tasks.
// Empty filter fields do not restrict the task list.
func (c *Client) ListTasks(ctx context.Context, filter TaskListFilter) (TaskListResult, error) {
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
	for key, value := range map[string]string{
		"due_from": filter.DueFrom, "due_to": filter.DueTo, "completion": filter.Completion,
		"sort": filter.Sort, "direction": filter.Direction,
	} {
		if value != "" {
			query.Set(key, value)
		}
	}
	if filter.Overdue {
		query.Set("overdue", "true")
	}
	if filter.Undated {
		query.Set("undated", "true")
	}
	if filter.Limit != 0 {
		query.Set("limit", fmt.Sprint(filter.Limit))
	}
	if filter.Offset != 0 {
		query.Set("offset", fmt.Sprint(filter.Offset))
	}

	var result TaskListResult
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/v1/tasks",
		query,
		nil,
		&result,
	); err != nil {
		return TaskListResult{}, err
	}
	return result, nil
}

func (c *Client) CreateTask(
	ctx context.Context,
	request TaskWriteRequest,
) (Task, error) {
	var task Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/v1/tasks",
		nil,
		request,
		&task,
	); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (c *Client) TodayTasks(ctx context.Context, options TaskDateQuery) (TodayTaskResult, error) {
	query := taskDateValues(options, false)
	var result TodayTaskResult
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/v1/tasks/today",
		query,
		nil,
		&result,
	); err != nil {
		return TodayTaskResult{}, err
	}
	return result, nil
}

func (c *Client) WeekTasks(ctx context.Context, options TaskDateQuery) (WeekTaskResult, error) {
	query := taskDateValues(options, true)
	var result WeekTaskResult
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/v1/tasks/week",
		query,
		nil,
		&result,
	); err != nil {
		return WeekTaskResult{}, err
	}
	return result, nil
}

func (c *Client) UpdateTask(
	ctx context.Context,
	id int,
	request TaskWriteRequest,
) (Task, error) {
	var task Task
	if err := c.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/tasks/%d", id),
		nil,
		request,
		&task,
	); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (c *Client) ToggleTask(ctx context.Context, id int, expectedRevision *int) (Task, error) {
	var task Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/toggle", id),
		nil,
		map[string]any{"expected_revision": expectedRevision},
		&task,
	); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (c *Client) ToggleTaskSubtask(
	ctx context.Context,
	todoID int,
	subtaskID int,
	expectedRevision *int,
) (Task, error) {
	var task Task
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/tasks/%d/subtasks/%d/toggle", todoID, subtaskID),
		nil,
		map[string]any{"expected_revision": expectedRevision},
		&task,
	); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (c *Client) DeleteTask(ctx context.Context, id int, expectedRevision *int) (TaskDeletionReceipt, error) {
	var receipt TaskDeletionReceipt
	query := make(url.Values)
	if expectedRevision != nil {
		query.Set("expected_revision", fmt.Sprint(*expectedRevision))
	}
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/tasks/%d", id),
		query,
		nil,
		&receipt,
	); err != nil {
		return TaskDeletionReceipt{}, err
	}
	return receipt, nil
}

func taskDateValues(options TaskDateQuery, includeWeekStart bool) url.Values {
	query := make(url.Values)
	if options.IncludeOverdue {
		query.Set("include_overdue", "true")
	}
	if options.IncludeUndated {
		query.Set("include_undated", "true")
	}
	if includeWeekStart && options.WeekStart != nil {
		query.Set("week_start", fmt.Sprint(*options.WeekStart))
	}
	return query
}
