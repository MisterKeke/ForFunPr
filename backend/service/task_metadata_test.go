package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func newFeatureTestService(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx := context.Background()
	if err := configureSQLite(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := NewService()
	service.db = db
	service.ctx = ctx
	service.ready = true
	return service
}

func TestRenameFavoriteCategoryPreservesAssignmentsAndRejectsConflicts(t *testing.T) {
	service := newFeatureTestService(t)
	category, err := service.CreateFavoriteCategory("News", "telegram")
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateFavoriteCategory("Research", "telegram")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.db.Exec(
		`INSERT INTO telegram_favorites (username, category_id) VALUES (?, ?)`,
		"example", category.ID,
	); err != nil {
		t.Fatal(err)
	}

	renamed, err := service.RenameFavoriteCategory(category.ID, "Headlines")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != category.ID || renamed.Name != "Headlines" {
		t.Fatalf("renamed category = %#v", renamed)
	}
	var assignedID int
	if err := service.db.QueryRow(
		`SELECT category_id FROM telegram_favorites WHERE username = ?`,
		"example",
	).Scan(&assignedID); err != nil {
		t.Fatal(err)
	}
	if assignedID != category.ID {
		t.Fatalf("assigned category ID = %d, want %d", assignedID, category.ID)
	}

	_, err = service.RenameFavoriteCategory(category.ID, other.Name)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("rename conflict error = %v, want ConflictError", err)
	}
}

func TestExistingTodoReturnsEmptyMetadata(t *testing.T) {
	service := newFeatureTestService(t)
	if _, err := service.db.Exec(`INSERT INTO todos (title) VALUES ('Existing task')`); err != nil {
		t.Fatal(err)
	}
	todos, err := service.GetTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("todo count = %d, want 1", len(todos))
	}
	if todos[0].Difficulty != "" || todos[0].Tags == nil || len(todos[0].Tags) != 0 ||
		todos[0].Subtasks == nil || len(todos[0].Subtasks) != 0 {
		t.Fatalf("existing todo metadata = difficulty %q, tags %#v, subtasks %#v",
			todos[0].Difficulty, todos[0].Tags, todos[0].Subtasks)
	}
}

func TestHardTodoMetadataIsPreservedValidatedAndToggleable(t *testing.T) {
	service := newFeatureTestService(t)
	todos, err := service.CreateTodo(TodoCreateRequest{
		Title:      "Ship release",
		Difficulty: "hard",
		Tags:       []string{"Release", "release", "Backend"},
		Subtasks: []TodoSubtaskInput{
			{Title: "Prepare changelog"},
			{Title: "Publish binaries"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	created := todos[0]
	if created.Difficulty != "hard" || len(created.Tags) != 2 || len(created.Subtasks) != 2 {
		t.Fatalf("created todo metadata = %#v", created)
	}

	todos, err = service.UpdateTodo(TodoUpdateRequest{
		ID:          created.ID,
		Title:       "Ship stable release",
		Description: created.Description,
		Priority:    created.Priority,
		DueDate:     created.DueDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated := todos[0]
	if updated.Difficulty != "hard" || len(updated.Tags) != 2 || len(updated.Subtasks) != 2 {
		t.Fatalf("preserved todo metadata = %#v", updated)
	}

	todos, err = service.ToggleTodoSubtask(TodoSubtaskIDRequest{
		TodoID: created.ID, SubtaskID: updated.Subtasks[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !todos[0].Subtasks[0].Done {
		t.Fatal("subtask was not toggled")
	}

	medium := "medium"
	_, err = service.UpdateTodo(TodoUpdateRequest{
		ID: created.ID, Title: updated.Title, Priority: updated.Priority,
		Difficulty: &medium,
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("difficulty change error = %v, want ValidationError", err)
	}

	emptySubtasks := []TodoSubtaskInput{}
	todos, err = service.UpdateTodo(TodoUpdateRequest{
		ID: created.ID, Title: updated.Title, Priority: updated.Priority,
		Difficulty: &medium, Subtasks: &emptySubtasks,
	})
	if err != nil {
		t.Fatal(err)
	}
	if todos[0].Difficulty != "medium" || len(todos[0].Subtasks) != 0 {
		t.Fatalf("cleared todo metadata = %#v", todos[0])
	}
}

func TestSearchTodosCombinesTextMetadataAndExactFilters(t *testing.T) {
	service := newFeatureTestService(t)
	requests := []TodoCreateRequest{
		{
			Title: "Ship release", Description: "Prepare the stable rollout",
			DueDate: "2026-08-01", Priority: "high", Difficulty: "hard",
			Tags: []string{"Backend", "Urgent"},
			Subtasks: []TodoSubtaskInput{{Title: "Publish binaries"}},
		},
		{
			Title: "Write documentation", DueDate: "2026-08-01",
			Priority: "low", Difficulty: "easy", Tags: []string{"Docs", "Urgent"},
		},
		{
			Title: "Triage backlog", Priority: "medium", Tags: []string{"Backend"},
		},
		{
			Title: "Reach 100% coverage", Priority: "medium",
		},
	}
	for _, request := range requests {
		if _, err := service.CreateTodo(request); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name   string
		filter TodoFilter
		want   string
	}{
		{
			name: "subtask text",
			filter: TodoFilter{Query: "binaries"},
			want: "Ship release",
		},
		{
			name: "combined fields",
			filter: TodoFilter{
				Query: "rollout", DueDate: "2026-08-01", Priority: "HIGH",
				Difficulty: "hard", Tags: []string{"backend", "URGENT"},
			},
			want: "Ship release",
		},
		{
			name: "all tags",
			filter: TodoFilter{Tags: []string{"Backend", "Urgent"}},
			want: "Ship release",
		},
		{
			name: "unset difficulty",
			filter: TodoFilter{Difficulty: "unset", Tags: []string{"backend"}},
			want: "Triage backlog",
		},
		{
			name: "literal wildcard",
			filter: TodoFilter{Query: "%"},
			want: "Reach 100% coverage",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			todos, err := service.SearchTodos(testCase.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(todos) != 1 || todos[0].Title != testCase.want {
				t.Fatalf("filtered todos = %#v, want only %q", todos, testCase.want)
			}
		})
	}

	_, err := service.SearchTodos(TodoFilter{Difficulty: "impossible"})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "difficulty" {
		t.Fatalf("invalid filter error = %v, want difficulty ValidationError", err)
	}
}
