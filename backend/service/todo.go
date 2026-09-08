package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	todosChangedEvent       = "todos:changed"
	maximumTodoSearchLength = 256
	defaultTodoLimit        = 100
	maximumTodoLimit        = 200
	todoDateLayout          = "2006-01-02"
)

// Todo is the canonical todo payload returned to the frontend.
//
// JSON fields use snake_case. In particular, title and description are the
// only names for the task text and details respectively.
type Todo struct {
	ID          int           `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Done        bool          `json:"done"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	Revision    int           `json:"revision"`
	DueDate     string        `json:"due_date"`
	DueState    string        `json:"due_state"`
	Priority    string        `json:"priority"`
	Difficulty  string        `json:"difficulty"`
	Tags        []string      `json:"tags"`
	Subtasks    []TodoSubtask `json:"subtasks"`
}

// TodoFilter describes the optional criteria supported by task searches.
// Tags use AND semantics: a task must contain every requested tag.
type TodoFilter struct {
	Query      string   `json:"query"`
	DueDate    string   `json:"due_date"`
	DueFrom    string   `json:"due_from"`
	DueTo      string   `json:"due_to"`
	Priority   string   `json:"priority"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags"`
	Completion string   `json:"completion"`
	Overdue    bool     `json:"overdue"`
	Undated    bool     `json:"undated"`
	Sort       string   `json:"sort"`
	Direction  string   `json:"direction"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
}

type TodoListResult struct {
	Items     []Todo `json:"items"`
	Total     int    `json:"total"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	HasMore   bool   `json:"has_more"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type TodoTodayQuery struct {
	IncludeOverdue bool `json:"include_overdue"`
	IncludeUndated bool `json:"include_undated"`
}

type TodoTodayResult struct {
	Overdue     []Todo `json:"overdue"`
	DueToday    []Todo `json:"due_today"`
	Unscheduled []Todo `json:"unscheduled"`
	Date        string `json:"date"`
	TimeZone    string `json:"time_zone"`
}

type TodoWeekQuery struct {
	IncludeOverdue bool `json:"include_overdue"`
	IncludeUndated bool `json:"include_undated"`
	WeekStart      *int `json:"week_start,omitempty"`
}

type TodoWeekResult struct {
	Items       []Todo `json:"items"`
	Overdue     []Todo `json:"overdue"`
	Unscheduled []Todo `json:"unscheduled"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	WeekStart   int    `json:"week_start"`
	TimeZone    string `json:"time_zone"`
}

type TodoSubtask struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int    `json:"position"`
}

type TodoSubtaskInput struct {
	ID       int    `json:"id,omitempty"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Position int    `json:"position,omitempty"`
}

// TodoCreateRequest is the typed input for CreateTodo.
type TodoCreateRequest struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Priority    string             `json:"priority"`
	DueDate     string             `json:"due_date"`
	Difficulty  string             `json:"difficulty"`
	Tags        []string           `json:"tags"`
	Subtasks    []TodoSubtaskInput `json:"subtasks"`
}

// TodoUpdateRequest is the typed input for UpdateTodo.
type TodoUpdateRequest struct {
	ID               int                 `json:"id"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	Priority         string              `json:"priority"`
	DueDate          string              `json:"due_date"`
	Difficulty       *string             `json:"difficulty,omitempty"`
	Tags             *[]string           `json:"tags,omitempty"`
	Subtasks         *[]TodoSubtaskInput `json:"subtasks,omitempty"`
	ExpectedRevision *int                `json:"expected_revision,omitempty"`
}

type TodoNotFoundError struct {
	ID int
}

func (e *TodoNotFoundError) Error() string {
	return fmt.Sprintf("todo with ID %d does not exist", e.ID)
}

type TodoIDRequest struct {
	ID               int  `json:"id"`
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

type TodoSubtaskIDRequest struct {
	TodoID           int  `json:"todo_id"`
	SubtaskID        int  `json:"subtask_id"`
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

type TodoDeletionReceipt struct {
	DeletedID       int `json:"deleted_id"`
	DeletedRevision int `json:"deleted_revision"`
}

type todoQueryStore interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (a *Service) GetTodos() ([]Todo, error) {
	return a.GetTodosContext(a.requestContext())
}

func (a *Service) GetTodosContext(ctx context.Context) ([]Todo, error) {
	return a.collectTodoPagesContext(ctx, TodoFilter{})
}

func queryTodos(ctx context.Context, store todoQueryStore) ([]Todo, error) {
	result, err := queryTodoListWithFilter(ctx, store, TodoFilter{}, time.Now())
	return result.Items, err
}

func (a *Service) SearchTodos(filter TodoFilter) ([]Todo, error) {
	return a.SearchTodosContext(a.requestContext(), filter)
}

func (a *Service) SearchTodosContext(ctx context.Context, filter TodoFilter) ([]Todo, error) {
	if filter.Limit == 0 && filter.Offset == 0 {
		return a.collectTodoPagesContext(ctx, filter)
	}
	result, err := a.ListTodosContext(ctx, filter)
	return result.Items, err
}

// collectTodoPagesContext preserves the legacy slice-returning Wails methods
// without allowing any individual SQL query or relation hydration pass to be
// unbounded. New interfaces should use ListTodosContext directly.
func (a *Service) collectTodoPagesContext(ctx context.Context, filter TodoFilter) ([]Todo, error) {
	filter.Limit = maximumTodoLimit
	filter.Offset = 0
	items := []Todo{}
	for {
		result, err := a.ListTodosContext(ctx, filter)
		if err != nil {
			return nil, err
		}
		items = append(items, result.Items...)
		if !result.HasMore || len(result.Items) == 0 {
			return items, nil
		}
		filter.Offset += len(result.Items)
	}
}

func (a *Service) ListTodosContext(ctx context.Context, filter TodoFilter) (TodoListResult, error) {
	return queryTodoListWithFilter(ctx, a.db, filter, a.now())
}

func queryTodoListWithFilter(
	ctx context.Context,
	store todoQueryStore,
	filter TodoFilter,
	now time.Time,
) (TodoListResult, error) {
	normalized, err := normalizeTodoFilter(filter)
	if err != nil {
		return TodoListResult{}, err
	}

	var where strings.Builder
	where.WriteString("1 = 1")
	args := []any{}

	if normalized.Query != "" {
		pattern := todoLikePattern(normalized.Query)
		where.WriteString(`
			AND (
				LOWER(COALESCE(t.title, '')) LIKE ? ESCAPE '\'
				OR LOWER(COALESCE(t.description, '')) LIKE ? ESCAPE '\'
				OR EXISTS (
					SELECT 1
					FROM todo_tags AS search_todo_tags
					JOIN tags AS search_tags ON search_tags.id = search_todo_tags.tag_id
					WHERE search_todo_tags.todo_id = t.id
					  AND search_tags.name_normalized LIKE ? ESCAPE '\'
				)
				OR EXISTS (
					SELECT 1
					FROM todo_subtasks AS search_subtasks
					WHERE search_subtasks.todo_id = t.id
					  AND LOWER(search_subtasks.title) LIKE ? ESCAPE '\'
				)
			)
		`)
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if normalized.DueDate != "" {
		where.WriteString(" AND t.due_date = ?")
		args = append(args, normalized.DueDate)
	}
	if normalized.DueFrom != "" {
		where.WriteString(" AND t.due_date >= ?")
		args = append(args, normalized.DueFrom)
	}
	if normalized.DueTo != "" {
		where.WriteString(" AND t.due_date <= ?")
		args = append(args, normalized.DueTo)
	}
	if normalized.Overdue {
		where.WriteString(" AND t.is_completed = 0 AND t.due_date IS NOT NULL AND t.due_date < ?")
		args = append(args, now.Format(todoDateLayout))
	}
	if normalized.Undated {
		where.WriteString(" AND t.due_date IS NULL")
	}
	if normalized.Completion == "complete" {
		where.WriteString(" AND t.is_completed <> 0")
	} else if normalized.Completion == "incomplete" {
		where.WriteString(" AND t.is_completed = 0")
	}
	if normalized.Priority != "" {
		where.WriteString(" AND t.priority = ?")
		args = append(args, normalized.Priority)
	}
	if normalized.Difficulty == "unset" {
		where.WriteString(" AND (t.difficulty IS NULL OR t.difficulty = '')")
	} else if normalized.Difficulty != "" {
		where.WriteString(" AND t.difficulty = ?")
		args = append(args, normalized.Difficulty)
	}
	for _, tag := range normalized.Tags {
		where.WriteString(`
			AND EXISTS (
				SELECT 1
				FROM todo_tags AS filter_todo_tags
				JOIN tags AS filter_tags ON filter_tags.id = filter_todo_tags.tag_id
				WHERE filter_todo_tags.todo_id = t.id
				  AND filter_tags.name_normalized = ?
			)
		`)
		args = append(args, strings.ToLower(tag))
	}

	var total int
	if err := store.QueryRowContext(ctx, "SELECT COUNT(*) FROM todos AS t WHERE "+where.String(), args...).Scan(&total); err != nil {
		return TodoListResult{}, fmt.Errorf("count todos: %w", err)
	}

	orderExpression := map[string]string{
		"created_at": "t.created_at",
		"updated_at": "t.updated_at",
		"due_date":   "CASE WHEN t.due_date IS NULL THEN 1 ELSE 0 END, t.due_date",
		"priority":   "CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END",
		"title":      "LOWER(t.title)",
		"id":         "t.id",
	}[normalized.Sort]
	direction := strings.ToUpper(normalized.Direction)
	query := `
		SELECT t.id, t.title, t.description, t.is_completed, t.created_at,
		       t.updated_at, t.revision, t.due_date, t.priority, t.difficulty
		FROM todos AS t
		WHERE ` + where.String() + `
		ORDER BY ` + orderExpression + ` ` + direction + `, t.id ` + direction + `
		LIMIT ? OFFSET ?`
	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, normalized.Limit, normalized.Offset)
	rows, err := store.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return TodoListResult{}, fmt.Errorf("search todos: %w", err)
	}
	todos, err := scanTodos(rows)
	if err != nil {
		return TodoListResult{}, err
	}
	todos, err = hydrateTodoRelations(ctx, store, todos)
	if err != nil {
		return TodoListResult{}, err
	}
	applyTodoDueStates(todos, now.Format(todoDateLayout))
	return TodoListResult{
		Items: todos, Total: total, Limit: normalized.Limit, Offset: normalized.Offset,
		HasMore: normalized.Offset+len(todos) < total,
		Sort:    normalized.Sort, Direction: normalized.Direction,
	}, nil
}

func (a *Service) GetTodayIncompleteTodos() ([]Todo, error) {
	return a.GetTodayIncompleteTodosContext(a.requestContext())
}

func (a *Service) GetTodayIncompleteTodosContext(ctx context.Context) ([]Todo, error) {
	result, err := a.GetTodayTodosContext(ctx, TodoTodayQuery{})
	return result.DueToday, err
}

func (a *Service) GetTodayTodosContext(ctx context.Context, request TodoTodayQuery) (TodoTodayResult, error) {
	now := a.now()
	today := now.Format(todoDateLayout)
	dueToday, err := a.ListTodosContext(ctx, TodoFilter{
		DueDate: today, Completion: "incomplete", Sort: "priority", Direction: "asc", Limit: maximumTodoLimit,
	})
	if err != nil {
		return TodoTodayResult{}, err
	}
	result := TodoTodayResult{DueToday: dueToday.Items, Overdue: []Todo{}, Unscheduled: []Todo{}, Date: today, TimeZone: now.Location().String()}
	if request.IncludeOverdue {
		overdue, err := a.ListTodosContext(ctx, TodoFilter{Overdue: true, Sort: "due_date", Direction: "asc", Limit: maximumTodoLimit})
		if err != nil {
			return TodoTodayResult{}, err
		}
		result.Overdue = overdue.Items
	}
	if request.IncludeUndated {
		undated, err := a.ListTodosContext(ctx, TodoFilter{Undated: true, Completion: "incomplete", Sort: "priority", Direction: "asc", Limit: maximumTodoLimit})
		if err != nil {
			return TodoTodayResult{}, err
		}
		result.Unscheduled = undated.Items
	}
	return result, nil
}

func (a *Service) GetThisWeekIncompleteTodos() ([]Todo, error) {
	return a.GetThisWeekIncompleteTodosContext(a.requestContext())
}

// GetThisWeekIncompleteTodosContext returns incomplete tasks due after today
// through the end of the current local week. The context-aware form is used
// by REST, CLI, and MCP calls so shutdown and request cancellation propagate
// into the database operation.
func (a *Service) GetThisWeekIncompleteTodosContext(ctx context.Context) ([]Todo, error) {
	result, err := a.GetThisWeekTodosContext(ctx, TodoWeekQuery{})
	return result.Items, err
}

func (a *Service) GetThisWeekTodosContext(ctx context.Context, request TodoWeekQuery) (TodoWeekResult, error) {
	now := a.now()
	weekStart, err := a.todoWeekStartContext(ctx)
	if err != nil {
		return TodoWeekResult{}, err
	}
	if request.WeekStart != nil {
		if *request.WeekStart < 0 || *request.WeekStart > 6 {
			return TodoWeekResult{}, &ValidationError{Field: "week_start", Message: "week start must be between 0 (Sunday) and 6 (Saturday)"}
		}
		weekStart = *request.WeekStart
	}
	daysSinceStart := (int(now.Weekday()) - weekStart + 7) % 7
	end := now.AddDate(0, 0, 6-daysSinceStart)
	start := now.AddDate(0, 0, 1)
	result := TodoWeekResult{
		Items: []Todo{}, Overdue: []Todo{}, Unscheduled: []Todo{},
		StartDate: start.Format(todoDateLayout), EndDate: end.Format(todoDateLayout),
		WeekStart: weekStart, TimeZone: now.Location().String(),
	}
	if !start.After(end) {
		items, err := a.ListTodosContext(ctx, TodoFilter{
			DueFrom: result.StartDate, DueTo: result.EndDate, Completion: "incomplete",
			Sort: "due_date", Direction: "asc", Limit: maximumTodoLimit,
		})
		if err != nil {
			return TodoWeekResult{}, err
		}
		result.Items = items.Items
	}
	if request.IncludeOverdue {
		overdue, err := a.ListTodosContext(ctx, TodoFilter{Overdue: true, Sort: "due_date", Direction: "asc", Limit: maximumTodoLimit})
		if err != nil {
			return TodoWeekResult{}, err
		}
		result.Overdue = overdue.Items
	}
	if request.IncludeUndated {
		undated, err := a.ListTodosContext(ctx, TodoFilter{Undated: true, Completion: "incomplete", Sort: "priority", Direction: "asc", Limit: maximumTodoLimit})
		if err != nil {
			return TodoWeekResult{}, err
		}
		result.Unscheduled = undated.Items
	}
	return result, nil
}

type TodoDatePreferences struct {
	WeekStart int    `json:"week_start"`
	TimeZone  string `json:"time_zone"`
}

func (a *Service) GetTodoDatePreferencesContext(ctx context.Context) (TodoDatePreferences, error) {
	weekStart, err := a.todoWeekStartContext(ctx)
	if err != nil {
		return TodoDatePreferences{}, err
	}
	return TodoDatePreferences{WeekStart: weekStart, TimeZone: a.now().Location().String()}, nil
}

func (a *Service) SetTodoDatePreferencesContext(ctx context.Context, preferences TodoDatePreferences) (TodoDatePreferences, error) {
	if preferences.WeekStart < 0 || preferences.WeekStart > 6 {
		return TodoDatePreferences{}, &ValidationError{Field: "week_start", Message: "week start must be between 0 (Sunday) and 6 (Saturday)"}
	}
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO app_state (key, value) VALUES ('todos.week_start', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, fmt.Sprint(preferences.WeekStart)); err != nil {
		return TodoDatePreferences{}, fmt.Errorf("save todo date preferences: %w", err)
	}
	return a.GetTodoDatePreferencesContext(ctx)
}

func (a *Service) todoWeekStartContext(ctx context.Context) (int, error) {
	var value string
	err := a.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = 'todos.week_start'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return int(time.Monday), nil
	}
	if err != nil {
		return 0, fmt.Errorf("load todo week start: %w", err)
	}
	var parsed int
	if _, err := fmt.Sscan(value, &parsed); err != nil || parsed < 0 || parsed > 6 {
		return int(time.Monday), nil
	}
	return parsed, nil
}

// GetTodosByDueDate returns every task due on the requested calendar date.
func (a *Service) GetTodosByDueDate(dueDate string) ([]Todo, error) {
	return a.GetTodosByDueDateContext(a.requestContext(), dueDate)
}

func (a *Service) GetTodosByDueDateContext(ctx context.Context, dueDate string) ([]Todo, error) {
	if strings.TrimSpace(dueDate) == "" {
		return []Todo{}, fmt.Errorf("due date is required")
	}
	return a.collectTodoPagesContext(ctx, TodoFilter{DueDate: dueDate})
}

func scanTodos(rows *sql.Rows) ([]Todo, error) {
	defer rows.Close()
	todos := []Todo{}
	for rows.Next() {
		var todo Todo
		var isCompleted int
		var title sql.NullString
		var description sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString
		var dueDate sql.NullString
		var priority sql.NullString
		var difficulty sql.NullString

		if err := rows.Scan(
			&todo.ID,
			&title,
			&description,
			&isCompleted,
			&createdAt,
			&updatedAt,
			&todo.Revision,
			&dueDate,
			&priority,
			&difficulty,
		); err != nil {
			return []Todo{}, fmt.Errorf("scan todo: %w", err)
		}

		todo.Title = title.String
		todo.Description = description.String
		todo.Done = isCompleted != 0
		todo.CreatedAt = createdAt.String
		todo.UpdatedAt = updatedAt.String
		todo.DueDate = dueDate.String
		todo.Priority = priority.String
		todo.Difficulty = difficulty.String
		todo.Tags = []string{}
		todo.Subtasks = []TodoSubtask{}

		todos = append(todos, todo)
	}

	if err := rows.Err(); err != nil {
		return []Todo{}, fmt.Errorf("iterate todos: %w", err)
	}

	return todos, nil
}

func applyTodoDueStates(todos []Todo, today string) {
	for index := range todos {
		switch {
		case todos[index].DueDate == "":
			todos[index].DueState = "unscheduled"
		case !todos[index].Done && todos[index].DueDate < today:
			todos[index].DueState = "overdue"
		case todos[index].DueDate == today:
			todos[index].DueState = "due_today"
		default:
			todos[index].DueState = "upcoming"
		}
	}
}

func hydrateTodoRelations(ctx context.Context, store todoQueryStore, todos []Todo) ([]Todo, error) {
	if len(todos) == 0 {
		return todos, nil
	}

	indexes := make(map[int]int, len(todos))
	for index := range todos {
		indexes[todos[index].ID] = index
	}

	ids := make([]int, 0, len(todos))
	for _, todo := range todos {
		ids = append(ids, todo.ID)
	}
	const relationChunkSize = 400
	for start := 0; start < len(ids); start += relationChunkSize {
		end := start + relationChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		arguments := make([]any, len(chunk))
		for index, id := range chunk {
			arguments[index] = id
		}
		tagRows, err := store.QueryContext(ctx, `
			SELECT todo_tags.todo_id, tags.name
			FROM todo_tags
			JOIN tags ON tags.id = todo_tags.tag_id
			WHERE todo_tags.todo_id IN (`+placeholders+`)
			ORDER BY todo_tags.todo_id, LOWER(tags.name), tags.id
		`, arguments...)
		if err != nil {
			return []Todo{}, fmt.Errorf("query todo tags: %w", err)
		}
		for tagRows.Next() {
			var todoID int
			var name string
			if err := tagRows.Scan(&todoID, &name); err != nil {
				tagRows.Close()
				return []Todo{}, fmt.Errorf("scan todo tag: %w", err)
			}
			if index, exists := indexes[todoID]; exists {
				todos[index].Tags = append(todos[index].Tags, name)
			}
		}
		if err := tagRows.Err(); err != nil {
			tagRows.Close()
			return []Todo{}, fmt.Errorf("iterate todo tags: %w", err)
		}
		if err := tagRows.Close(); err != nil {
			return []Todo{}, fmt.Errorf("close todo tag rows: %w", err)
		}

		subtaskRows, err := store.QueryContext(ctx, `
			SELECT id, todo_id, title, is_completed, position
			FROM todo_subtasks
			WHERE todo_id IN (`+placeholders+`)
			ORDER BY todo_id, position, id
		`, arguments...)
		if err != nil {
			return []Todo{}, fmt.Errorf("query todo subtasks: %w", err)
		}
		for subtaskRows.Next() {
			var todoID int
			var completed int
			var subtask TodoSubtask
			if err := subtaskRows.Scan(&subtask.ID, &todoID, &subtask.Title, &completed, &subtask.Position); err != nil {
				subtaskRows.Close()
				return []Todo{}, fmt.Errorf("scan todo subtask: %w", err)
			}
			subtask.Done = completed != 0
			if index, exists := indexes[todoID]; exists {
				todos[index].Subtasks = append(todos[index].Subtasks, subtask)
			}
		}
		if err := subtaskRows.Err(); err != nil {
			subtaskRows.Close()
			return []Todo{}, fmt.Errorf("iterate todo subtasks: %w", err)
		}
		if err := subtaskRows.Close(); err != nil {
			return []Todo{}, fmt.Errorf("close todo subtask rows: %w", err)
		}
	}
	return todos, nil
}

func normalizeTodoFilter(filter TodoFilter) (TodoFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > maximumTodoSearchLength {
		return TodoFilter{}, &ValidationError{
			Field:   "query",
			Message: fmt.Sprintf("task search cannot exceed %d characters", maximumTodoSearchLength),
		}
	}

	filter.DueDate = strings.TrimSpace(filter.DueDate)
	if filter.DueDate != "" {
		normalizedDate, err := normalizeDueDate(filter.DueDate)
		if err != nil {
			return TodoFilter{}, &ValidationError{
				Field: "date", Message: "date must use YYYY-MM-DD",
			}
		}
		filter.DueDate = normalizedDate.(string)
	}
	for field, value := range map[string]*string{"due_from": &filter.DueFrom, "due_to": &filter.DueTo} {
		*value = strings.TrimSpace(*value)
		if *value == "" {
			continue
		}
		normalizedDate, err := normalizeDueDate(*value)
		if err != nil {
			return TodoFilter{}, &ValidationError{Field: field, Message: field + " must use YYYY-MM-DD"}
		}
		*value = normalizedDate.(string)
	}
	if filter.DueFrom != "" && filter.DueTo != "" && filter.DueFrom > filter.DueTo {
		return TodoFilter{}, &ValidationError{Field: "due_from", Message: "due_from cannot be after due_to"}
	}
	if filter.DueDate != "" && (filter.DueFrom != "" || filter.DueTo != "" || filter.Overdue || filter.Undated) {
		return TodoFilter{}, &ValidationError{Field: "due_date", Message: "due_date cannot be combined with due ranges, overdue, or undated"}
	}
	if filter.Overdue && filter.Undated {
		return TodoFilter{}, &ValidationError{Field: "overdue", Message: "overdue and undated cannot both be selected"}
	}
	if filter.Undated && (filter.DueFrom != "" || filter.DueTo != "") {
		return TodoFilter{}, &ValidationError{Field: "undated", Message: "undated cannot be combined with a due-date range"}
	}

	filter.Priority = strings.ToLower(strings.TrimSpace(filter.Priority))
	if filter.Priority != "" {
		switch filter.Priority {
		case "low", "medium", "high":
		default:
			return TodoFilter{}, &ValidationError{
				Field: "priority", Message: "priority must be low, medium, or high",
			}
		}
	}

	filter.Difficulty = strings.ToLower(strings.TrimSpace(filter.Difficulty))
	switch filter.Difficulty {
	case "", "unset", "easy", "medium", "hard":
	default:
		return TodoFilter{}, &ValidationError{
			Field: "difficulty", Message: "difficulty must be unset, easy, medium, or hard",
		}
	}
	filter.Completion = strings.ToLower(strings.TrimSpace(filter.Completion))
	if filter.Completion == "" {
		filter.Completion = "all"
	}
	switch filter.Completion {
	case "all", "complete", "incomplete":
	default:
		return TodoFilter{}, &ValidationError{Field: "completion", Message: "completion must be all, complete, or incomplete"}
	}
	filter.Sort = strings.ToLower(strings.TrimSpace(filter.Sort))
	if filter.Sort == "" {
		filter.Sort = "created_at"
	}
	switch filter.Sort {
	case "created_at", "updated_at", "due_date", "priority", "title", "id":
	default:
		return TodoFilter{}, &ValidationError{Field: "sort", Message: "unsupported task ordering"}
	}
	filter.Direction = strings.ToLower(strings.TrimSpace(filter.Direction))
	if filter.Direction == "" {
		filter.Direction = "desc"
	}
	if filter.Direction != "asc" && filter.Direction != "desc" {
		return TodoFilter{}, &ValidationError{Field: "direction", Message: "direction must be asc or desc"}
	}
	if filter.Limit == 0 {
		filter.Limit = defaultTodoLimit
	}
	if filter.Limit < 1 || filter.Limit > maximumTodoLimit {
		return TodoFilter{}, &ValidationError{Field: "limit", Message: fmt.Sprintf("limit must be between 1 and %d", maximumTodoLimit)}
	}
	if filter.Offset < 0 {
		return TodoFilter{}, &ValidationError{Field: "offset", Message: "offset cannot be negative"}
	}

	if filter.Tags == nil {
		filter.Tags = []string{}
	} else {
		tags, err := normalizeTodoTags(filter.Tags)
		if err != nil {
			return TodoFilter{}, err
		}
		filter.Tags = tags
	}
	return filter, nil
}

func todoLikePattern(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return "%" + strings.ToLower(replacer.Replace(value)) + "%"
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

func normalizeTodoDifficulty(difficulty string) (string, error) {
	difficulty = strings.ToLower(strings.TrimSpace(difficulty))
	switch difficulty {
	case "", "easy", "medium", "hard":
		return difficulty, nil
	default:
		return "", &ValidationError{
			Field: "difficulty", Message: "difficulty must be easy, medium, or hard",
		}
	}
}

func NormalizeTodoDifficulty(difficulty string) (string, error) {
	return normalizeTodoDifficulty(difficulty)
}

func normalizeTodoTags(values []string) ([]string, error) {
	tags := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, &ValidationError{Field: "tags", Message: "tag names cannot be empty"}
		}
		if len([]rune(name)) > 64 {
			return nil, &ValidationError{Field: "tags", Message: "tag names cannot exceed 64 characters"}
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, name)
	}
	if len(tags) > 32 {
		return nil, &ValidationError{Field: "tags", Message: "a task cannot have more than 32 tags"}
	}
	return tags, nil
}

func NormalizeTodoTags(values []string) ([]string, error) {
	return normalizeTodoTags(values)
}

func normalizeTodoSubtasks(
	values []TodoSubtaskInput,
	difficulty string,
) ([]TodoSubtaskInput, error) {
	if len(values) > 0 && difficulty != "hard" {
		return nil, &ValidationError{
			Field: "subtasks", Message: "subtasks are only allowed for hard tasks",
		}
	}
	if len(values) > 100 {
		return nil, &ValidationError{
			Field: "subtasks", Message: "a task cannot have more than 100 subtasks",
		}
	}

	normalized := make([]TodoSubtaskInput, 0, len(values))
	seenIDs := make(map[int]struct{}, len(values))
	for index, value := range values {
		if value.ID < 0 {
			return nil, &ValidationError{Field: "subtasks", Message: "subtask IDs cannot be negative"}
		}
		if value.ID > 0 {
			if _, exists := seenIDs[value.ID]; exists {
				return nil, &ValidationError{Field: "subtasks", Message: "subtask IDs cannot be repeated"}
			}
			seenIDs[value.ID] = struct{}{}
		}
		value.Title = strings.TrimSpace(value.Title)
		if value.Title == "" {
			return nil, &ValidationError{Field: "subtasks", Message: "subtask titles cannot be empty"}
		}
		value.Position = index
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func difficultyDatabaseValue(difficulty string) any {
	if difficulty == "" {
		return nil
	}
	return difficulty
}

func replaceTodoTagsContext(
	ctx context.Context,
	tx *sql.Tx,
	todoID int,
	tags []string,
) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM todo_tags WHERE todo_id = ?`, todoID); err != nil {
		return fmt.Errorf("clear todo tags: %w", err)
	}
	for _, name := range tags {
		key := strings.ToLower(name)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tags (name, name_normalized)
			VALUES (?, ?)
			ON CONFLICT(name_normalized) DO NOTHING
		`, name, key); err != nil {
			return fmt.Errorf("create todo tag: %w", err)
		}

		var tagID int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT id FROM tags WHERE name_normalized = ?`,
			key,
		).Scan(&tagID); err != nil {
			return fmt.Errorf("load todo tag: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO todo_tags (todo_id, tag_id)
			VALUES (?, ?)
		`, todoID, tagID); err != nil {
			return fmt.Errorf("assign todo tag: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM tags
		WHERE NOT EXISTS (
			SELECT 1 FROM todo_tags WHERE todo_tags.tag_id = tags.id
		)
	`); err != nil {
		return fmt.Errorf("remove unused todo tags: %w", err)
	}
	return nil
}

func replaceTodoSubtasksContext(
	ctx context.Context,
	tx *sql.Tx,
	todoID int,
	subtasks []TodoSubtaskInput,
) error {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id FROM todo_subtasks WHERE todo_id = ?`,
		todoID,
	)
	if err != nil {
		return fmt.Errorf("query existing todo subtasks: %w", err)
	}
	existing := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing todo subtask: %w", err)
		}
		existing[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate existing todo subtasks: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close existing todo subtask rows: %w", err)
	}

	retained := make(map[int]struct{}, len(subtasks))
	for _, subtask := range subtasks {
		completed := 0
		if subtask.Done {
			completed = 1
		}
		if subtask.ID == 0 {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO todo_subtasks (
					todo_id, title, is_completed, position
				) VALUES (?, ?, ?, ?)
			`, todoID, subtask.Title, completed, subtask.Position); err != nil {
				return fmt.Errorf("create todo subtask: %w", err)
			}
			continue
		}
		if _, exists := existing[subtask.ID]; !exists {
			return &ValidationError{
				Field:   "subtasks",
				Message: fmt.Sprintf("subtask %d does not belong to this task", subtask.ID),
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE todo_subtasks
			SET title = ?, is_completed = ?, position = ?
			WHERE id = ? AND todo_id = ?
		`, subtask.Title, completed, subtask.Position, subtask.ID, todoID); err != nil {
			return fmt.Errorf("update todo subtask: %w", err)
		}
		retained[subtask.ID] = struct{}{}
	}

	for id := range existing {
		if _, keep := retained[id]; keep {
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM todo_subtasks WHERE id = ? AND todo_id = ?`,
			id,
			todoID,
		); err != nil {
			return fmt.Errorf("delete todo subtask: %w", err)
		}
	}
	return nil
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

func (a *Service) commitTodoMutationAndLoadContext(ctx context.Context, tx *sql.Tx, id int) (Todo, error) {
	if err := tx.Commit(); err != nil {
		return Todo{}, fmt.Errorf("commit todo mutation: %w", err)
	}
	return a.GetTodoContext(ctx, id)
}

func (a *Service) GetTodoContext(ctx context.Context, id int) (Todo, error) {
	if err := validateTodoID(id); err != nil {
		return Todo{}, err
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, title, description, is_completed, created_at, updated_at,
		       revision, due_date, priority, difficulty
		FROM todos WHERE id = ?
	`, id)
	if err != nil {
		return Todo{}, fmt.Errorf("load todo: %w", err)
	}
	items, err := scanTodos(rows)
	if err != nil {
		return Todo{}, err
	}
	if len(items) == 0 {
		return Todo{}, &TodoNotFoundError{ID: id}
	}
	items, err = hydrateTodoRelations(ctx, a.db, items)
	if err != nil {
		return Todo{}, err
	}
	applyTodoDueStates(items, a.now().Format(todoDateLayout))
	return items[0], nil
}

func checkExpectedTodoRevision(expected *int, actual int, id int) error {
	if expected == nil {
		return nil
	}
	if *expected <= 0 {
		return &ValidationError{Field: "expected_revision", Message: "expected revision must be positive"}
	}
	if *expected != actual {
		return &StaleRevisionError{Resource: "todo", ID: id, Expected: *expected, Actual: actual}
	}
	return nil
}

func NormalizeDate(value string, required bool) (string, error) {
	normalized, err := normalizeDueDate(value)
	if err != nil {
		return "", err
	}
	if normalized == nil {
		if required {
			return "", &ValidationError{Field: "date", Message: "date is required"}
		}
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

func (a *Service) CreateTodo(request TodoCreateRequest) (Todo, error) {
	return a.CreateTodoContext(a.requestContext(), request)
}

func (a *Service) CreateTodoContext(ctx context.Context, request TodoCreateRequest) (Todo, error) {
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return Todo{}, fmt.Errorf("todo title cannot be empty")
	}

	dueDate, err := normalizeDueDate(request.DueDate)
	if err != nil {
		return Todo{}, err
	}
	priority, err := normalizeTodoPriority(request.Priority)
	if err != nil {
		return Todo{}, err
	}
	difficulty, err := normalizeTodoDifficulty(request.Difficulty)
	if err != nil {
		return Todo{}, err
	}
	tags, err := normalizeTodoTags(request.Tags)
	if err != nil {
		return Todo{}, err
	}
	subtasks, err := normalizeTodoSubtasks(request.Subtasks, difficulty)
	if err != nil {
		return Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Todo{}, fmt.Errorf("begin create todo: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO todos (title, description, is_completed, priority, due_date, difficulty, revision, updated_at)
		 VALUES (?, ?, 0, ?, ?, ?, 1, CURRENT_TIMESTAMP)`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
		difficultyDatabaseValue(difficulty),
	)
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "create todo", 0); err != nil {
		return Todo{}, err
	}
	createdID, err := result.LastInsertId()
	if err != nil {
		return Todo{}, fmt.Errorf("read created todo ID: %w", err)
	}
	if err := replaceTodoTagsContext(ctx, tx, int(createdID), tags); err != nil {
		return Todo{}, err
	}
	if err := replaceTodoSubtasksContext(ctx, tx, int(createdID), subtasks); err != nil {
		return Todo{}, err
	}

	return a.commitTodoMutationAndLoadContext(ctx, tx, int(createdID))
}

func (a *Service) UpdateTodo(request TodoUpdateRequest) (Todo, error) {
	return a.UpdateTodoContext(a.requestContext(), request)
}

func (a *Service) UpdateTodoContext(ctx context.Context, request TodoUpdateRequest) (Todo, error) {
	if err := validateTodoID(request.ID); err != nil {
		return Todo{}, err
	}

	title := strings.TrimSpace(request.Title)
	if title == "" {
		return Todo{}, fmt.Errorf("todo title cannot be empty")
	}

	dueDate, err := normalizeDueDate(request.DueDate)
	if err != nil {
		return Todo{}, err
	}
	priority, err := normalizeTodoPriority(request.Priority)
	if err != nil {
		return Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Todo{}, fmt.Errorf("begin update todo: %w", err)
	}
	defer tx.Rollback()

	var currentDifficulty sql.NullString
	var currentRevision int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT difficulty, revision FROM todos WHERE id = ?`,
		request.ID,
	).Scan(&currentDifficulty, &currentRevision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Todo{}, &TodoNotFoundError{ID: request.ID}
		}
		return Todo{}, fmt.Errorf("load todo state: %w", err)
	}
	if err := checkExpectedTodoRevision(request.ExpectedRevision, currentRevision, request.ID); err != nil {
		return Todo{}, err
	}
	difficulty := currentDifficulty.String
	if request.Difficulty != nil {
		difficulty, err = normalizeTodoDifficulty(*request.Difficulty)
		if err != nil {
			return Todo{}, err
		}
	}

	var tags []string
	if request.Tags != nil {
		tags, err = normalizeTodoTags(*request.Tags)
		if err != nil {
			return Todo{}, err
		}
	}
	var subtasks []TodoSubtaskInput
	if request.Subtasks != nil {
		subtasks, err = normalizeTodoSubtasks(*request.Subtasks, difficulty)
		if err != nil {
			return Todo{}, err
		}
	} else if difficulty != "hard" {
		var count int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM todo_subtasks WHERE todo_id = ?`,
			request.ID,
		).Scan(&count); err != nil {
			return Todo{}, fmt.Errorf("count todo subtasks: %w", err)
		}
		if count > 0 {
			return Todo{}, &ValidationError{
				Field:   "subtasks",
				Message: "clear subtasks before changing difficulty away from hard",
			}
		}
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE todos
		 SET title = ?, description = ?, priority = ?, due_date = ?, difficulty = ?,
		     revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND revision = ?`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
		difficultyDatabaseValue(difficulty),
		request.ID,
		currentRevision,
	)
	if err != nil {
		return Todo{}, fmt.Errorf("update todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "update todo", request.ID); err != nil {
		return Todo{}, err
	}
	if request.Tags != nil {
		if err := replaceTodoTagsContext(ctx, tx, request.ID, tags); err != nil {
			return Todo{}, err
		}
	}
	if request.Subtasks != nil {
		if err := replaceTodoSubtasksContext(ctx, tx, request.ID, subtasks); err != nil {
			return Todo{}, err
		}
	}

	return a.commitTodoMutationAndLoadContext(ctx, tx, request.ID)
}

func (a *Service) ToggleTodo(request TodoIDRequest) (Todo, error) {
	return a.ToggleTodoContext(a.requestContext(), request)
}

func (a *Service) ToggleTodoContext(ctx context.Context, request TodoIDRequest) (Todo, error) {
	if err := validateTodoID(request.ID); err != nil {
		return Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Todo{}, fmt.Errorf("begin toggle todo: %w", err)
	}
	defer tx.Rollback()
	var revision int
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM todos WHERE id = ?`, request.ID).Scan(&revision); errors.Is(err, sql.ErrNoRows) {
		return Todo{}, &TodoNotFoundError{ID: request.ID}
	} else if err != nil {
		return Todo{}, fmt.Errorf("load todo revision: %w", err)
	}
	if err := checkExpectedTodoRevision(request.ExpectedRevision, revision, request.ID); err != nil {
		return Todo{}, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE todos
		SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END,
		    revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, request.ID, revision)
	if err != nil {
		return Todo{}, fmt.Errorf("toggle todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "toggle todo", request.ID); err != nil {
		return Todo{}, err
	}

	return a.commitTodoMutationAndLoadContext(ctx, tx, request.ID)
}

func (a *Service) ToggleTodoSubtask(request TodoSubtaskIDRequest) (Todo, error) {
	return a.ToggleTodoSubtaskContext(a.requestContext(), request)
}

func (a *Service) ToggleTodoSubtaskContext(
	ctx context.Context,
	request TodoSubtaskIDRequest,
) (Todo, error) {
	if err := validateTodoID(request.TodoID); err != nil {
		return Todo{}, err
	}
	if request.SubtaskID <= 0 {
		return Todo{}, &ValidationError{
			Field: "subtask_id", Message: "subtask ID must be a positive integer",
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Todo{}, fmt.Errorf("begin toggle todo subtask: %w", err)
	}
	defer tx.Rollback()
	var revision int
	var difficulty sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT revision, difficulty FROM todos WHERE id = ?`, request.TodoID).Scan(&revision, &difficulty); errors.Is(err, sql.ErrNoRows) {
		return Todo{}, &TodoNotFoundError{ID: request.TodoID}
	} else if err != nil {
		return Todo{}, fmt.Errorf("load todo revision: %w", err)
	}
	if err := checkExpectedTodoRevision(request.ExpectedRevision, revision, request.TodoID); err != nil {
		return Todo{}, err
	}
	if difficulty.String != "hard" {
		return Todo{}, &ValidationError{Field: "todo_id", Message: "subtasks are only available for hard tasks"}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE todo_subtasks
		SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END
		WHERE id = ? AND todo_id = ?
	`, request.SubtaskID, request.TodoID)
	if err != nil {
		return Todo{}, fmt.Errorf("toggle todo subtask: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Todo{}, fmt.Errorf("check toggle todo subtask result: %w", err)
	}
	if affected == 0 {
		return Todo{}, &NotFoundError{
			Resource: "todo subtask", Key: fmt.Sprint(request.SubtaskID),
		}
	}
	parentResult, err := tx.ExecContext(ctx, `
		UPDATE todos SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, request.TodoID, revision)
	if err != nil {
		return Todo{}, fmt.Errorf("update todo after subtask toggle: %w", err)
	}
	if err := requireSingleTodoMutation(parentResult, "update todo after subtask toggle", request.TodoID); err != nil {
		return Todo{}, err
	}
	return a.commitTodoMutationAndLoadContext(ctx, tx, request.TodoID)
}

func (a *Service) DeleteTodo(request TodoIDRequest) (TodoDeletionReceipt, error) {
	return a.DeleteTodoContext(a.requestContext(), request)
}

func (a *Service) DeleteTodoContext(ctx context.Context, request TodoIDRequest) (TodoDeletionReceipt, error) {
	if err := validateTodoID(request.ID); err != nil {
		return TodoDeletionReceipt{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return TodoDeletionReceipt{}, fmt.Errorf("begin delete todo: %w", err)
	}
	defer tx.Rollback()
	var revision int
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM todos WHERE id = ?`, request.ID).Scan(&revision); errors.Is(err, sql.ErrNoRows) {
		return TodoDeletionReceipt{}, &TodoNotFoundError{ID: request.ID}
	} else if err != nil {
		return TodoDeletionReceipt{}, fmt.Errorf("load todo revision: %w", err)
	}
	if err := checkExpectedTodoRevision(request.ExpectedRevision, revision, request.ID); err != nil {
		return TodoDeletionReceipt{}, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM todos WHERE id = ? AND revision = ?`, request.ID, revision)
	if err != nil {
		return TodoDeletionReceipt{}, fmt.Errorf("delete todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "delete todo", request.ID); err != nil {
		return TodoDeletionReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return TodoDeletionReceipt{}, fmt.Errorf("commit delete todo: %w", err)
	}
	return TodoDeletionReceipt{DeletedID: request.ID, DeletedRevision: revision}, nil
}
