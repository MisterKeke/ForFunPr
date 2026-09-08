package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	backend "something/backend/service"
)

type taskResponse struct {
	ID          int                   `json:"id"`
	DueDate     string                `json:"due_date"`
	Title       string                `json:"title"`
	Description string                `json:"description,omitempty"`
	Priority    string                `json:"priority,omitempty"`
	Done        bool                  `json:"done"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
	Revision    int                   `json:"revision"`
	DueState    string                `json:"due_state"`
	Difficulty  string                `json:"difficulty"`
	Tags        []string              `json:"tags"`
	Subtasks    []backend.TodoSubtask `json:"subtasks"`
}

type taskWriteRequest struct {
	Title            string                      `json:"title"`
	Description      string                      `json:"description"`
	Priority         string                      `json:"priority"`
	DueDate          string                      `json:"due_date"`
	Difficulty       *string                     `json:"difficulty,omitempty"`
	Tags             *[]string                   `json:"tags,omitempty"`
	Subtasks         *[]backend.TodoSubtaskInput `json:"subtasks,omitempty"`
	ExpectedRevision *int                        `json:"expected_revision,omitempty"`
}

type taskRevisionRequest struct {
	ExpectedRevision *int `json:"expected_revision,omitempty"`
}

type taskListResponse struct {
	Items     []taskResponse `json:"items"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
	HasMore   bool           `json:"has_more"`
	Sort      string         `json:"sort"`
	Direction string         `json:"direction"`
}

func tasksHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		filter := backend.TodoFilter{
			Query:      strings.TrimSpace(r.URL.Query().Get("q")),
			DueDate:    strings.TrimSpace(r.URL.Query().Get("date")),
			Priority:   strings.TrimSpace(r.URL.Query().Get("priority")),
			Difficulty: strings.TrimSpace(r.URL.Query().Get("difficulty")),
			Tags:       r.URL.Query()["tag"],
			DueFrom:    strings.TrimSpace(r.URL.Query().Get("due_from")),
			DueTo:      strings.TrimSpace(r.URL.Query().Get("due_to")),
			Completion: strings.TrimSpace(r.URL.Query().Get("completion")),
			Sort:       strings.TrimSpace(r.URL.Query().Get("sort")),
			Direction:  strings.TrimSpace(r.URL.Query().Get("direction")),
		}
		if value := strings.TrimSpace(r.URL.Query().Get("overdue")); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_overdue", "Overdue must be true or false.")
				return
			}
			filter.Overdue = parsed
		}
		if value := strings.TrimSpace(r.URL.Query().Get("undated")); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_undated", "Undated must be true or false.")
				return
			}
			filter.Undated = parsed
		}
		for name, destination := range map[string]*int{"limit": &filter.Limit, "offset": &filter.Offset} {
			value := strings.TrimSpace(r.URL.Query().Get(name))
			if value == "" {
				continue
			}
			parsed, err := strconv.Atoi(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_pagination", "Task pagination values must be integers.")
				return
			}
			*destination = parsed
		}
		if filter.DueDate != "" && !validDate(filter.DueDate) {
			writeError(w, http.StatusBadRequest, "invalid_date", "The date must use YYYY-MM-DD.")
			return
		}

		result, err := app.ListTodosContext(r.Context(), filter)
		if err != nil {
			var validation *backend.ValidationError
			if errors.As(err, &validation) {
				writeError(w, http.StatusBadRequest, "invalid_task_filter", validation.Message)
				return
			}
			writeError(w, http.StatusInternalServerError, "tasks_failed", "Tasks could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, taskListResponse{
			Items: taskResponses(result.Items), Total: result.Total, Limit: result.Limit,
			Offset: result.Offset, HasMore: result.HasMore, Sort: result.Sort, Direction: result.Direction,
		})
	}
}

func todayTasksHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		query, ok := parseTaskDateQuery(w, r, false)
		if !ok {
			return
		}
		result, err := app.GetTodayTodosContext(r.Context(), backend.TodoTodayQuery{
			IncludeOverdue: query.IncludeOverdue, IncludeUndated: query.IncludeUndated,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "today_tasks_failed", "Today's tasks could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"overdue": taskResponses(result.Overdue), "due_today": taskResponses(result.DueToday),
			"unscheduled": taskResponses(result.Unscheduled), "date": result.Date, "time_zone": result.TimeZone,
		})
	}
}

func thisWeekTasksHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		query, ok := parseTaskDateQuery(w, r, true)
		if !ok {
			return
		}
		result, err := app.GetThisWeekTodosContext(r.Context(), backend.TodoWeekQuery{
			IncludeOverdue: query.IncludeOverdue, IncludeUndated: query.IncludeUndated, WeekStart: query.WeekStart,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "this_week_tasks_failed", "This week's remaining tasks could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"items": taskResponses(result.Items), "overdue": taskResponses(result.Overdue),
			"unscheduled": taskResponses(result.Unscheduled), "start_date": result.StartDate,
			"end_date": result.EndDate, "week_start": result.WeekStart, "time_zone": result.TimeZone,
		})
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
			UpdatedAt:   todo.UpdatedAt,
			Revision:    todo.Revision,
			DueState:    todo.DueState,
			Difficulty:  todo.Difficulty,
			Tags:        todo.Tags,
			Subtasks:    todo.Subtasks,
		})
	}

	return items
}

func createTaskHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		var request taskWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		if !validateTaskWriteRequest(w, &request) {
			return
		}

		difficulty := ""
		if request.Difficulty != nil {
			difficulty = *request.Difficulty
		}
		tags := []string{}
		if request.Tags != nil {
			tags = *request.Tags
		}
		subtasks := []backend.TodoSubtaskInput{}
		if request.Subtasks != nil {
			subtasks = *request.Subtasks
		}
		todo, err := app.CreateTodoContext(r.Context(), backend.TodoCreateRequest{
			Title:       request.Title,
			Description: request.Description,
			Priority:    request.Priority,
			DueDate:     request.DueDate,
			Difficulty:  difficulty,
			Tags:        tags,
			Subtasks:    subtasks,
		})
		if err != nil {
			writeTaskMutationError(
				w,
				err,
				"task_create_failed",
				"Task could not be created.",
			)
			return
		}
		app.EmitTodosChanged()

		writeJSON(w, http.StatusCreated, taskResponses([]backend.Todo{todo})[0])
	}
}

func updateTaskHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		id, ok := parseTaskID(w, r)
		if !ok {
			return
		}

		var request taskWriteRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		if !validateTaskWriteRequest(w, &request) {
			return
		}

		todo, err := app.UpdateTodoContext(r.Context(), backend.TodoUpdateRequest{
			ID:               id,
			Title:            request.Title,
			Description:      request.Description,
			Priority:         request.Priority,
			DueDate:          request.DueDate,
			Difficulty:       request.Difficulty,
			Tags:             request.Tags,
			Subtasks:         request.Subtasks,
			ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeTaskMutationError(
				w,
				err,
				"task_update_failed",
				"Task could not be updated.",
			)
			return
		}
		app.EmitTodosChanged()

		writeJSON(w, http.StatusOK, taskResponses([]backend.Todo{todo})[0])
	}
}

func toggleTaskHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		id, ok := parseTaskID(w, r)
		if !ok {
			return
		}

		// Requiring {} with application/json helps prevent simple
		// cross-origin form requests from mutating this localhost API.
		var request taskRevisionRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}

		todo, err := app.ToggleTodoContext(r.Context(), backend.TodoIDRequest{ID: id, ExpectedRevision: request.ExpectedRevision})
		if err != nil {
			writeTaskMutationError(
				w,
				err,
				"task_toggle_failed",
				"Task completion could not be changed.",
			)
			return
		}
		app.EmitTodosChanged()

		writeJSON(w, http.StatusOK, taskResponses([]backend.Todo{todo})[0])
	}
}

func toggleTaskSubtaskHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		todoID, ok := parseTaskID(w, r)
		if !ok {
			return
		}
		subtaskID, err := strconv.Atoi(strings.TrimSpace(r.PathValue("subtaskID")))
		if err != nil || subtaskID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_subtask_id", "Subtask ID must be a positive integer.")
			return
		}
		var request taskRevisionRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}

		todo, err := app.ToggleTodoSubtaskContext(r.Context(), backend.TodoSubtaskIDRequest{
			TodoID: todoID, SubtaskID: subtaskID, ExpectedRevision: request.ExpectedRevision,
		})
		if err != nil {
			writeTaskMutationError(w, err, "subtask_toggle_failed", "Subtask completion could not be changed.")
			return
		}
		app.EmitTodosChanged()
		writeJSON(w, http.StatusOK, taskResponses([]backend.Todo{todo})[0])
	}
}

func deleteTaskHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		id, ok := parseTaskID(w, r)
		if !ok {
			return
		}

		var expectedRevision *int
		if value := strings.TrimSpace(r.URL.Query().Get("expected_revision")); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				writeError(w, http.StatusBadRequest, "invalid_expected_revision", "Expected revision must be a positive integer.")
				return
			}
			expectedRevision = &parsed
		}
		receipt, err := app.DeleteTodoContext(r.Context(), backend.TodoIDRequest{ID: id, ExpectedRevision: expectedRevision})
		if err != nil {
			writeTaskMutationError(
				w,
				err,
				"task_delete_failed",
				"Task could not be deleted.",
			)
			return
		}
		app.EmitTodosChanged()

		writeJSON(w, http.StatusOK, receipt)
	}
}

type parsedTaskDateQuery struct {
	IncludeOverdue bool
	IncludeUndated bool
	WeekStart      *int
}

func parseTaskDateQuery(w http.ResponseWriter, r *http.Request, allowWeekStart bool) (parsedTaskDateQuery, bool) {
	var result parsedTaskDateQuery
	for name, destination := range map[string]*bool{
		"include_overdue": &result.IncludeOverdue,
		"include_undated": &result.IncludeUndated,
	} {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_task_date_filter", name+" must be true or false.")
			return parsedTaskDateQuery{}, false
		}
		*destination = parsed
	}
	if allowWeekStart {
		if value := strings.TrimSpace(r.URL.Query().Get("week_start")); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || parsed > 6 {
				writeError(w, http.StatusBadRequest, "invalid_week_start", "Week start must be between 0 (Sunday) and 6 (Saturday).")
				return parsedTaskDateQuery{}, false
			}
			result.WeekStart = &parsed
		}
	}
	return result, true
}

func parseTaskID(
	w http.ResponseWriter,
	r *http.Request,
) (int, bool) {
	value := strings.TrimSpace(r.PathValue("id"))

	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_task_id",
			"Task ID must be a positive integer.",
		)
		return 0, false
	}

	return id, true
}

func validateTaskWriteRequest(
	w http.ResponseWriter,
	request *taskWriteRequest,
) bool {
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	request.DueDate = strings.TrimSpace(request.DueDate)
	request.Priority = strings.ToLower(strings.TrimSpace(request.Priority))
	if request.Difficulty != nil {
		value := strings.ToLower(strings.TrimSpace(*request.Difficulty))
		request.Difficulty = &value
		if _, err := backend.NormalizeTodoDifficulty(value); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_task_difficulty", "Difficulty must be easy, medium, or hard.")
			return false
		}
	}
	if request.Tags != nil {
		tags, err := backend.NormalizeTodoTags(*request.Tags)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_task_tags", err.Error())
			return false
		}
		request.Tags = &tags
	}

	if request.Title == "" {
		writeError(
			w,
			http.StatusUnprocessableEntity,
			"invalid_task_title",
			"Task title cannot be empty.",
		)
		return false
	}

	if _, err := backend.NormalizeDate(request.DueDate, false); err != nil {
		writeError(
			w,
			http.StatusUnprocessableEntity,
			"invalid_due_date",
			"Due date must use YYYY-MM-DD.",
		)
		return false
	}

	normalizedPriority, err := backend.NormalizeTodoPriority(request.Priority)
	if err != nil {
		writeError(
			w,
			http.StatusUnprocessableEntity,
			"invalid_task_priority",
			"Priority must be low, medium, or high.",
		)
		return false
	}
	request.Priority = normalizedPriority
	return true
}

func writeTaskMutationError(
	w http.ResponseWriter,
	err error,
	code string,
	message string,
) {
	var notFound *backend.TodoNotFoundError
	if errors.As(err, &notFound) {
		writeError(
			w,
			http.StatusNotFound,
			"task_not_found",
			"The requested task does not exist.",
		)
		return
	}
	var resourceNotFound *backend.NotFoundError
	if errors.As(err, &resourceNotFound) {
		writeError(w, http.StatusNotFound, "subtask_not_found", "The requested subtask does not exist for this task.")
		return
	}
	var validation *backend.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_task", validation.Message)
		return
	}
	var stale *backend.StaleRevisionError
	if errors.As(err, &stale) {
		writeError(w, http.StatusConflict, "stale_task_revision", "The task changed since it was loaded. Refresh it and try again.")
		return
	}

	writeError(w, http.StatusInternalServerError, code, message)
}
