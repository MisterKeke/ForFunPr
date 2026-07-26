package backend

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const todosChangedEvent = "todos:changed"

// Todo is the canonical todo payload returned to the frontend.
//
// JSON fields use snake_case. In particular, title and description are the
// only names for the task text and details respectively.
type Todo struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
	CreatedAt   string `json:"created_at"`
	DueDate     string `json:"due_date"`
	Priority    string `json:"priority"`
}

// TodoCreateRequest is the typed input for CreateTodo.
type TodoCreateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

// TodoUpdateRequest is the typed input for UpdateTodo.
type TodoUpdateRequest struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

type TodoNotFoundError struct {
	ID int
}

func (e *TodoNotFoundError) Error() string {
	return fmt.Sprintf("todo with ID %d does not exist", e.ID)
}

type TodoIDRequest struct {
	ID int `json:"id"`
}

type todoQueryStore interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (a *Service) GetTodos() ([]Todo, error) {
	return a.GetTodosContext(a.requestContext())
}

func (a *Service) GetTodosContext(ctx context.Context) ([]Todo, error) {
	return queryTodos(ctx, a.db)
}

func queryTodos(ctx context.Context, store todoQueryStore) ([]Todo, error) {
	rows, err := store.QueryContext(ctx, `SELECT id, title, description, is_completed, created_at, due_date, priority FROM todos ORDER BY created_at DESC`)
	if err != nil {
		return []Todo{}, fmt.Errorf("query todos: %w", err)
	}
	defer rows.Close()

	return scanTodos(rows)
}

func (a *Service) GetTodayIncompleteTodos() ([]Todo, error) {
	today := time.Now().Format("2006-01-02")

	rows, err := a.db.Query(`
		SELECT id, title, description, is_completed, created_at, due_date, priority
		FROM todos
		WHERE is_completed = 0 AND due_date = ?
		ORDER BY
			CASE priority
				WHEN 'high' THEN 0
				WHEN 'medium' THEN 1
				WHEN 'low' THEN 2
				ELSE 3
			END,
			created_at ASC
	`, today)
	if err != nil {
		return []Todo{}, fmt.Errorf("query today's incomplete todos: %w", err)
	}
	defer rows.Close()

	return scanTodos(rows)
}

func (a *Service) GetThisWeekIncompleteTodos() ([]Todo, error) {
	now := time.Now()

	tomorrow := now.AddDate(0, 0, 1)

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	sunday := now.AddDate(0, 0, 7-weekday)

	if tomorrow.After(sunday) {
		return []Todo{}, nil
	}

	startOfWeek := tomorrow.Format("2006-01-02")
	endOfWeek := sunday.Format("2006-01-02")

	rows, err := a.db.Query(`
		SELECT id, title, description, is_completed, created_at, due_date, priority
		FROM todos
		WHERE is_completed = 0
		  AND due_date BETWEEN ? AND ?
		ORDER BY
			due_date ASC,
			CASE priority
				WHEN 'high' THEN 0
				WHEN 'medium' THEN 1
				WHEN 'low' THEN 2
				ELSE 3
			END,
			created_at ASC
	`, startOfWeek, endOfWeek)
	if err != nil {
		return []Todo{}, fmt.Errorf("query remaining week's incomplete todos: %w", err)
	}
	defer rows.Close()

	return scanTodos(rows)
}

// GetTodosByDueDate returns every task due on the requested calendar date.
func (a *Service) GetTodosByDueDate(dueDate string) ([]Todo, error) {
	normalizedDueDate, err := normalizeDueDate(dueDate)
	if err != nil {
		return []Todo{}, err
	}
	if normalizedDueDate == nil {
		return []Todo{}, fmt.Errorf("due date is required")
	}

	rows, err := a.db.Query(`
		SELECT id, title, description, is_completed, created_at, due_date, priority
		FROM todos
		WHERE due_date = ?
		ORDER BY
			CASE priority
				WHEN 'high' THEN 0
				WHEN 'medium' THEN 1
				WHEN 'low' THEN 2
				ELSE 3
			END,
			created_at ASC
	`, normalizedDueDate)
	if err != nil {
		return []Todo{}, fmt.Errorf("query todos by due date: %w", err)
	}
	defer rows.Close()

	return scanTodos(rows)
}

func scanTodos(rows *sql.Rows) ([]Todo, error) {
	todos := []Todo{}
	for rows.Next() {
		var todo Todo
		var isCompleted int
		var title sql.NullString
		var description sql.NullString
		var createdAt sql.NullString
		var dueDate sql.NullString
		var priority sql.NullString

		if err := rows.Scan(
			&todo.ID,
			&title,
			&description,
			&isCompleted,
			&createdAt,
			&dueDate,
			&priority,
		); err != nil {
			return []Todo{}, fmt.Errorf("scan todo: %w", err)
		}

		todo.Title = title.String
		todo.Description = description.String
		todo.Done = isCompleted != 0
		todo.CreatedAt = createdAt.String
		todo.DueDate = dueDate.String
		todo.Priority = priority.String

		todos = append(todos, todo)
	}

	if err := rows.Err(); err != nil {
		return []Todo{}, fmt.Errorf("iterate todos: %w", err)
	}

	return todos, nil
}

func normalizeTodoPriority(priority string) (string, error) {
	priority = strings.ToLower(strings.TrimSpace(priority))
	if priority == "" {
		return "medium", nil
	}
	switch priority {
	case "low", "medium", "high":
		return priority, nil
	default:
		return "", &ValidationError{Field: "priority", Message: "priority must be low, medium, or high"}
	}
}

func NormalizeTodoPriority(priority string) (string, error) {
	return normalizeTodoPriority(priority)
}

// normalizeDueDate validates the canonical YYYY-MM-DD input. An empty due
// date is stored as SQL NULL rather than an empty string.
func normalizeDueDate(dueDate string) (interface{}, error) {
	dueDate = strings.TrimSpace(dueDate)
	if dueDate == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", dueDate)
	if err != nil || parsed.Format("2006-01-02") != dueDate {
		return nil, fmt.Errorf("due date must use YYYY-MM-DD")
	}

	return parsed.Format("2006-01-02"), nil
}

func validateTodoID(id int) error {
	if id <= 0 {
		return fmt.Errorf("todo ID must be a positive integer")
	}
	return nil
}

func requireSingleTodoMutation(result sql.Result, operation string, id int) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check %s result: %w", operation, err)
	}
	if affected == 0 {
		if id == 0 {
			return fmt.Errorf("%s did not create a todo", operation)
		}

		return &TodoNotFoundError{ID: id}
	}
	if affected != 1 {
		return fmt.Errorf("%s for todo ID %d affected %d rows", operation, id, affected)
	}
	return nil
}

func getTodosAfterMutationContext(ctx context.Context, tx *sql.Tx) ([]Todo, error) {
	todos, err := queryTodos(ctx, tx)
	if err != nil { return nil, err }
	if err := tx.Commit(); err != nil { return nil, fmt.Errorf("commit todo mutation: %w", err) }
	return todos, nil
}

func NormalizeDate(value string, required bool) (string, error) {
	normalized, err := normalizeDueDate(value)
	if err != nil { return "", err }
	if normalized == nil {
		if required { return "", &ValidationError{Field: "date", Message: "date is required"} }
		return "", nil
	}
	return normalized.(string), nil
}

// EmitTodosChanged is called by the REST layer after an externally-originated
// mutation. UI mutations use their returned canonical list and do not emit a
// second backend event.
func (a *Service) EmitTodosChanged() {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, todosChangedEvent)
	}
}

func (a *Service) CreateTodo(request TodoCreateRequest) ([]Todo, error) {
	return a.CreateTodoContext(a.requestContext(), request)
}

func (a *Service) CreateTodoContext(ctx context.Context, request TodoCreateRequest) ([]Todo, error) {
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return []Todo{}, fmt.Errorf("todo title cannot be empty")
	}

	dueDate, err := normalizeDueDate(request.DueDate)
	if err != nil {
		return []Todo{}, err
	}
	priority, err := normalizeTodoPriority(request.Priority)
	if err != nil {
		return []Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin create todo: %w", err) }
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO todos (title, description, is_completed, priority, due_date) VALUES (?, ?, 0, ?, ?)`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
	)
	if err != nil {
		return []Todo{}, fmt.Errorf("create todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "create todo", 0); err != nil {
		return []Todo{}, err
	}

	return getTodosAfterMutationContext(ctx, tx)
}

func (a *Service) UpdateTodo(request TodoUpdateRequest) ([]Todo, error) {
	return a.UpdateTodoContext(a.requestContext(), request)
}

func (a *Service) UpdateTodoContext(ctx context.Context, request TodoUpdateRequest) ([]Todo, error) {
	if err := validateTodoID(request.ID); err != nil {
		return []Todo{}, err
	}

	title := strings.TrimSpace(request.Title)
	if title == "" {
		return []Todo{}, fmt.Errorf("todo title cannot be empty")
	}

	dueDate, err := normalizeDueDate(request.DueDate)
	if err != nil {
		return []Todo{}, err
	}
	priority, err := normalizeTodoPriority(request.Priority)
	if err != nil {
		return []Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin update todo: %w", err) }
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE todos SET title = ?, description = ?, priority = ?, due_date = ? WHERE id = ?`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
		request.ID,
	)
	if err != nil {
		return []Todo{}, fmt.Errorf("update todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "update todo", request.ID); err != nil {
		return []Todo{}, err
	}

	return getTodosAfterMutationContext(ctx, tx)
}

func (a *Service) ToggleTodo(request TodoIDRequest) ([]Todo, error) {
	return a.ToggleTodoContext(a.requestContext(), request)
}

func (a *Service) ToggleTodoContext(ctx context.Context, request TodoIDRequest) ([]Todo, error) {
	if err := validateTodoID(request.ID); err != nil {
		return []Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin toggle todo: %w", err) }
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE todos
		SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END
		WHERE id = ?
	`, request.ID)
	if err != nil {
		return []Todo{}, fmt.Errorf("toggle todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "toggle todo", request.ID); err != nil {
		return []Todo{}, err
	}

	return getTodosAfterMutationContext(ctx, tx)
}

func (a *Service) DeleteTodo(request TodoIDRequest) ([]Todo, error) {
	return a.DeleteTodoContext(a.requestContext(), request)
}

func (a *Service) DeleteTodoContext(ctx context.Context, request TodoIDRequest) ([]Todo, error) {
	if err := validateTodoID(request.ID); err != nil {
		return []Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin delete todo: %w", err) }
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM todos WHERE id = ?`, request.ID)
	if err != nil {
		return []Todo{}, fmt.Errorf("delete todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "delete todo", request.ID); err != nil {
		return []Todo{}, err
	}

	return getTodosAfterMutationContext(ctx, tx)
}
