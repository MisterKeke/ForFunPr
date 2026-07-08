package backend

import (
	"database/sql"
	"strings"
)

type ToDoItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Time      string `json:"time"`
	Details   string `json:"details"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
	DueDate   string `json:"due_date"`
	Priority  string `json:"priority"`
}

func (a *App) GetTodos() ([]ToDoItem, error) {
	rows, err := a.db.Query(`SELECT id, title, description, is_completed, created_at, due_date, priority FROM todos ORDER BY created_at DESC`)
	if err != nil {
		return []ToDoItem{}, err
	}
	defer rows.Close()

	result := []ToDoItem{}
	for rows.Next() {
		var t ToDoItem
		var isCompleted int
		var title sql.NullString
		var description sql.NullString
		var createdAt sql.NullString
		var dueDate sql.NullString
		var priority sql.NullString

		if err := rows.Scan(&t.ID, &title, &description, &isCompleted, &createdAt, &dueDate, &priority); err != nil {
			continue
		}
		t.Title = title.String
		t.Text = title.String
		t.Details = description.String
		t.Done = isCompleted != 0
		t.CreatedAt = createdAt.String
		t.DueDate = dueDate.String
		t.Priority = priority.String

		result = append(result, t)
	}

	return result, nil
}

func normalizeTodoPriority(priority string) string {
	priority = strings.ToLower(strings.TrimSpace(priority))
	switch priority {
	case "low", "medium", "high":
		return priority
	default:
		return "medium"
	}
}

func (a *App) CreateTodo(title string, description string, priority string) ([]ToDoItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return a.GetTodos()
	}

	description = strings.TrimSpace(description)
	priority = normalizeTodoPriority(priority)

	_, err := a.db.Exec(
		`INSERT INTO todos (title, description, is_completed, priority) VALUES (?, ?, 0, ?)`,
		title, description, priority,
	)
	if err != nil {
		return []ToDoItem{}, err
	}

	return a.GetTodos()
}

func (a *App) UpdateTodo(id int, title string, description string, priority string) ([]ToDoItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return a.GetTodos()
	}

	description = strings.TrimSpace(description)
	priority = normalizeTodoPriority(priority)

	_, err := a.db.Exec(
		`UPDATE todos SET title = ?, description = ?, priority = ? WHERE id = ?`,
		title, description, priority, id,
	)
	if err != nil {
		return []ToDoItem{}, err
	}

	return a.GetTodos()
}

func (a *App) ToggleTodo(id int) ([]ToDoItem, error) {
	// Get current value
	var cur int
	err := a.db.QueryRow(`SELECT is_completed FROM todos WHERE id = ?`, id).Scan(&cur)
	if err != nil {
		return a.GetTodos()
	}

	newVal := 1
	if cur != 0 {
		newVal = 0
	}

	_, err = a.db.Exec(`UPDATE todos SET is_completed = ? WHERE id = ?`, newVal, id)
	if err != nil {
		return a.GetTodos()
	}

	return a.GetTodos()
}

func (a *App) DeleteTodo(id int) ([]ToDoItem, error) {
	_, err := a.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return a.GetTodos()
	}
	return a.GetTodos()
}
