package backend

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
	DueDate     string        `json:"due_date"`
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
	Priority   string   `json:"priority"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags"`
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
	ID          int                 `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Priority    string              `json:"priority"`
	DueDate     string              `json:"due_date"`
	Difficulty  *string             `json:"difficulty,omitempty"`
	Tags        *[]string           `json:"tags,omitempty"`
	Subtasks    *[]TodoSubtaskInput `json:"subtasks,omitempty"`
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

type TodoSubtaskIDRequest struct {
	TodoID    int `json:"todo_id"`
	SubtaskID int `json:"subtask_id"`
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
	return queryTodosWithFilter(ctx, store, TodoFilter{})
}

func (a *Service) SearchTodos(filter TodoFilter) ([]Todo, error) {
	return a.SearchTodosContext(a.requestContext(), filter)
}

func (a *Service) SearchTodosContext(ctx context.Context, filter TodoFilter) ([]Todo, error) {
	return queryTodosWithFilter(ctx, a.db, filter)
}

func queryTodosWithFilter(
	ctx context.Context,
	store todoQueryStore,
	filter TodoFilter,
) ([]Todo, error) {
	normalized, err := normalizeTodoFilter(filter)
	if err != nil {
		return []Todo{}, err
	}

	var query strings.Builder
	query.WriteString(`
		SELECT t.id, t.title, t.description, t.is_completed, t.created_at,
		       t.due_date, t.priority, t.difficulty
		FROM todos AS t
		WHERE 1 = 1
	`)
	args := []any{}

	if normalized.Query != "" {
		pattern := todoLikePattern(normalized.Query)
		query.WriteString(`
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
		query.WriteString(" AND t.due_date = ?")
		args = append(args, normalized.DueDate)
	}
	if normalized.Priority != "" {
		query.WriteString(" AND t.priority = ?")
		args = append(args, normalized.Priority)
	}
	if normalized.Difficulty == "unset" {
		query.WriteString(" AND (t.difficulty IS NULL OR t.difficulty = '')")
	} else if normalized.Difficulty != "" {
		query.WriteString(" AND t.difficulty = ?")
		args = append(args, normalized.Difficulty)
	}
	for _, tag := range normalized.Tags {
		query.WriteString(`
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

	if normalized.DueDate != "" {
		query.WriteString(`
			ORDER BY
				CASE t.priority
					WHEN 'high' THEN 0
					WHEN 'medium' THEN 1
					WHEN 'low' THEN 2
					ELSE 3
				END,
				t.created_at ASC
		`)
	} else {
		query.WriteString(" ORDER BY t.created_at DESC")
	}

	rows, err := store.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return []Todo{}, fmt.Errorf("search todos: %w", err)
	}
	todos, err := scanTodos(rows)
	if err != nil {
		return []Todo{}, err
	}
	return hydrateTodoRelations(ctx, store, todos)
}

func (a *Service) GetTodayIncompleteTodos() ([]Todo, error) {
	today := time.Now().Format("2006-01-02")

	rows, err := a.db.Query(`
		SELECT id, title, description, is_completed, created_at, due_date, priority, difficulty
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
	todos, err := scanTodos(rows)
	if err != nil {
		return []Todo{}, err
	}
	return hydrateTodoRelations(a.requestContext(), a.db, todos)
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
		SELECT id, title, description, is_completed, created_at, due_date, priority, difficulty
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
	todos, err := scanTodos(rows)
	if err != nil {
		return []Todo{}, err
	}
	return hydrateTodoRelations(a.requestContext(), a.db, todos)
}

// GetTodosByDueDate returns every task due on the requested calendar date.
func (a *Service) GetTodosByDueDate(dueDate string) ([]Todo, error) {
	if strings.TrimSpace(dueDate) == "" {
		return []Todo{}, fmt.Errorf("due date is required")
	}
	return a.SearchTodos(TodoFilter{DueDate: dueDate})
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
		var dueDate sql.NullString
		var priority sql.NullString
		var difficulty sql.NullString

		if err := rows.Scan(
			&todo.ID,
			&title,
			&description,
			&isCompleted,
			&createdAt,
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

func hydrateTodoRelations(ctx context.Context, store todoQueryStore, todos []Todo) ([]Todo, error) {
	if len(todos) == 0 {
		return todos, nil
	}

	indexes := make(map[int]int, len(todos))
	for index := range todos {
		indexes[todos[index].ID] = index
	}

	tagRows, err := store.QueryContext(ctx, `
		SELECT todo_tags.todo_id, tags.name
		FROM todo_tags
		JOIN tags ON tags.id = todo_tags.tag_id
		ORDER BY todo_tags.todo_id, LOWER(tags.name), tags.id
	`)
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
		ORDER BY todo_id, position, id
	`)
	if err != nil {
		return []Todo{}, fmt.Errorf("query todo subtasks: %w", err)
	}
	defer subtaskRows.Close()
	for subtaskRows.Next() {
		var todoID int
		var completed int
		var subtask TodoSubtask
		if err := subtaskRows.Scan(
			&subtask.ID,
			&todoID,
			&subtask.Title,
			&completed,
			&subtask.Position,
		); err != nil {
			return []Todo{}, fmt.Errorf("scan todo subtask: %w", err)
		}
		subtask.Done = completed != 0
		if index, exists := indexes[todoID]; exists {
			todos[index].Subtasks = append(todos[index].Subtasks, subtask)
		}
	}
	if err := subtaskRows.Err(); err != nil {
		return []Todo{}, fmt.Errorf("iterate todo subtasks: %w", err)
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

func getTodosAfterMutationContext(ctx context.Context, tx *sql.Tx) ([]Todo, error) {
	todos, err := queryTodos(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit todo mutation: %w", err)
	}
	return todos, nil
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
	difficulty, err := normalizeTodoDifficulty(request.Difficulty)
	if err != nil {
		return []Todo{}, err
	}
	tags, err := normalizeTodoTags(request.Tags)
	if err != nil {
		return []Todo{}, err
	}
	subtasks, err := normalizeTodoSubtasks(request.Subtasks, difficulty)
	if err != nil {
		return []Todo{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create todo: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO todos (title, description, is_completed, priority, due_date, difficulty) VALUES (?, ?, 0, ?, ?, ?)`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
		difficultyDatabaseValue(difficulty),
	)
	if err != nil {
		return []Todo{}, fmt.Errorf("create todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "create todo", 0); err != nil {
		return []Todo{}, err
	}
	createdID, err := result.LastInsertId()
	if err != nil {
		return []Todo{}, fmt.Errorf("read created todo ID: %w", err)
	}
	if err := replaceTodoTagsContext(ctx, tx, int(createdID), tags); err != nil {
		return []Todo{}, err
	}
	if err := replaceTodoSubtasksContext(ctx, tx, int(createdID), subtasks); err != nil {
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
	if err != nil {
		return nil, fmt.Errorf("begin update todo: %w", err)
	}
	defer tx.Rollback()

	var currentDifficulty sql.NullString
	if err := tx.QueryRowContext(
		ctx,
		`SELECT difficulty FROM todos WHERE id = ?`,
		request.ID,
	).Scan(&currentDifficulty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []Todo{}, &TodoNotFoundError{ID: request.ID}
		}
		return []Todo{}, fmt.Errorf("load todo difficulty: %w", err)
	}
	difficulty := currentDifficulty.String
	if request.Difficulty != nil {
		difficulty, err = normalizeTodoDifficulty(*request.Difficulty)
		if err != nil {
			return []Todo{}, err
		}
	}

	var tags []string
	if request.Tags != nil {
		tags, err = normalizeTodoTags(*request.Tags)
		if err != nil {
			return []Todo{}, err
		}
	}
	var subtasks []TodoSubtaskInput
	if request.Subtasks != nil {
		subtasks, err = normalizeTodoSubtasks(*request.Subtasks, difficulty)
		if err != nil {
			return []Todo{}, err
		}
	} else if difficulty != "hard" {
		var count int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM todo_subtasks WHERE todo_id = ?`,
			request.ID,
		).Scan(&count); err != nil {
			return []Todo{}, fmt.Errorf("count todo subtasks: %w", err)
		}
		if count > 0 {
			return []Todo{}, &ValidationError{
				Field:   "subtasks",
				Message: "clear subtasks before changing difficulty away from hard",
			}
		}
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE todos SET title = ?, description = ?, priority = ?, due_date = ?, difficulty = ? WHERE id = ?`,
		title,
		strings.TrimSpace(request.Description),
		priority,
		dueDate,
		difficultyDatabaseValue(difficulty),
		request.ID,
	)
	if err != nil {
		return []Todo{}, fmt.Errorf("update todo: %w", err)
	}
	if err := requireSingleTodoMutation(result, "update todo", request.ID); err != nil {
		return []Todo{}, err
	}
	if request.Tags != nil {
		if err := replaceTodoTagsContext(ctx, tx, request.ID, tags); err != nil {
			return []Todo{}, err
		}
	}
	if request.Subtasks != nil {
		if err := replaceTodoSubtasksContext(ctx, tx, request.ID, subtasks); err != nil {
			return []Todo{}, err
		}
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
	if err != nil {
		return nil, fmt.Errorf("begin toggle todo: %w", err)
	}
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

func (a *Service) ToggleTodoSubtask(request TodoSubtaskIDRequest) ([]Todo, error) {
	return a.ToggleTodoSubtaskContext(a.requestContext(), request)
}

func (a *Service) ToggleTodoSubtaskContext(
	ctx context.Context,
	request TodoSubtaskIDRequest,
) ([]Todo, error) {
	if err := validateTodoID(request.TodoID); err != nil {
		return []Todo{}, err
	}
	if request.SubtaskID <= 0 {
		return []Todo{}, &ValidationError{
			Field: "subtask_id", Message: "subtask ID must be a positive integer",
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin toggle todo subtask: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE todo_subtasks
		SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END
		WHERE id = ? AND todo_id = ?
		  AND EXISTS (
			SELECT 1 FROM todos
			WHERE todos.id = todo_subtasks.todo_id
			  AND todos.difficulty = 'hard'
		  )
	`, request.SubtaskID, request.TodoID)
	if err != nil {
		return []Todo{}, fmt.Errorf("toggle todo subtask: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return []Todo{}, fmt.Errorf("check toggle todo subtask result: %w", err)
	}
	if affected == 0 {
		return []Todo{}, &NotFoundError{
			Resource: "todo subtask", Key: fmt.Sprint(request.SubtaskID),
		}
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
	if err != nil {
		return nil, fmt.Errorf("begin delete todo: %w", err)
	}
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
