package api

import (
	"net/http"
	"strings"

	"currency-wails/backend"
)

type taskResponse struct {
	ID          int    `json:"id"`
	DueDate     string `json:"due_date"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Done        bool   `json:"done"`
	CreatedAt   string `json:"created_at"`
}

func tasksHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		date := strings.TrimSpace(r.URL.Query().Get("date"))
		var (
			todos []backend.Todo
			err   error
		)
		if date == "" {
			todos, err = app.GetTodos()
		} else {
			if !validDate(date) {
				writeError(w, http.StatusBadRequest, "invalid_date", "The date must use YYYY-MM-DD.")
				return
			}
			todos, err = app.GetTodosByDueDate(date)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tasks_failed", "Tasks could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, taskResponses(todos))
	}
}

func todayTasksHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		todos, err := app.GetTodayIncompleteTodos()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "today_tasks_failed", "Today's tasks could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, taskResponses(todos))
	}
}

func taskResponses(todos []backend.Todo) []taskResponse {
	items := make([]taskResponse, 0, len(todos))
	for _, todo := range todos {
		items = append(items, taskResponse{
			ID:          todo.ID,
			DueDate:     todo.DueDate,
			Title:       todo.Title,
			Description: todo.Description,
			Priority:    todo.Priority,
			Done:        todo.Done,
			CreatedAt:   todo.CreatedAt,
		})
	}

	return items
}
