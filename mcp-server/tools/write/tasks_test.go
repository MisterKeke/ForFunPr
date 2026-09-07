package writetools

import (
	"reflect"
	"strings"
	"testing"

	"something/mcp-server/schemas"
)

func TestNormalizeTaskWrite(t *testing.T) {
	title, description, priority, dueDate, err := normalizeTaskWrite(
		"  ship release  ",
		"  notes  ",
		" HIGH ",
		" 2026-09-08 ",
	)
	if err != nil {
		t.Fatalf("normalizeTaskWrite returned %v", err)
	}
	if title != "ship release" || description != "notes" || priority != "high" || dueDate != "2026-09-08" {
		t.Fatalf("normalized values = %q, %q, %q, %q", title, description, priority, dueDate)
	}

	for name, values := range map[string][4]string{
		"missing title": {" ", "", "", ""},
		"bad priority":  {"title", "", "urgent", ""},
		"bad date":      {"title", "", "", "September 8"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, _, err := normalizeTaskWrite(values[0], values[1], values[2], values[3]); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeTaskMetadata(t *testing.T) {
	difficulty, tags, subtasks, err := normalizeTaskMetadata(
		" HARD ",
		[]string{" work ", "home"},
		[]schemas.TaskSubtaskInput{
			{Title: " first ", Position: 99},
			{ID: 3, Title: " second ", Done: true, Position: 99},
		},
	)
	if err != nil {
		t.Fatalf("normalizeTaskMetadata returned %v", err)
	}
	if difficulty != "hard" || !reflect.DeepEqual(tags, []string{"work", "home"}) {
		t.Fatalf("metadata = %q, %#v", difficulty, tags)
	}
	if len(subtasks) != 2 || subtasks[0].Title != "first" || subtasks[0].Position != 0 ||
		subtasks[1].Title != "second" || subtasks[1].Position != 1 {
		t.Fatalf("subtasks = %#v", subtasks)
	}

	tests := []struct {
		name       string
		difficulty string
		subtasks   []schemas.TaskSubtaskInput
		contains   string
	}{
		{"negative id", "hard", []schemas.TaskSubtaskInput{{ID: -1, Title: "one"}}, "negative"},
		{"duplicate id", "hard", []schemas.TaskSubtaskInput{{ID: 1, Title: "one"}, {ID: 1, Title: "two"}}, "repeated"},
		{"non-hard subtasks", "easy", []schemas.TaskSubtaskInput{{Title: "one"}}, "only allowed"},
		{"blank subtask", "hard", []schemas.TaskSubtaskInput{{Title: " "}}, "subtask.title"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := normalizeTaskMetadata(test.difficulty, nil, test.subtasks)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAppendTaskFlags(t *testing.T) {
	args := appendTaskOptionalFlags([]string{"tasks", "create"}, "description", "medium", "2026-09-08")
	want := []string{"tasks", "create", "--description", "description", "--priority", "medium", "--due-date", "2026-09-08"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("optional flags = %#v", args)
	}

	args, err := appendTaskMetadataFlags(
		[]string{"tasks", "update"},
		"hard",
		[]string{"work", "home"},
		[]schemas.TaskSubtaskInput{{Title: "first"}, {ID: 2, Title: "second", Done: true, Position: 1}},
	)
	if err != nil {
		t.Fatalf("appendTaskMetadataFlags returned %v", err)
	}
	want = []string{
		"tasks", "update", "--difficulty", "hard",
		"--tag", "work", "--tag", "home",
		"--subtasks-json", `[{"title":"first"},{"id":2,"title":"second","done":true,"position":1}]`,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("metadata flags = %#v", args)
	}

	args, err = appendTaskMetadataFlags([]string{"tasks"}, "", []string{}, nil)
	if err != nil || !reflect.DeepEqual(args, []string{"tasks", "--clear-tags"}) {
		t.Fatalf("clear tags flags = %#v, %v", args, err)
	}
}

func TestSetupWriteArgs(t *testing.T) {
	args, err := setupWriteArgs([]string{"setups", "create"}, " Daily ", " work ", []int{4, 2})
	if err != nil {
		t.Fatalf("setupWriteArgs returned %v", err)
	}
	want := []string{"setups", "create", "--name", "Daily", "--description", "work", "--app-id", "4", "--app-id", "2"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v", args)
	}

	for name, appIDs := range map[string][]int{
		"empty":       nil,
		"nonpositive": {0},
		"duplicate":   {1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := setupWriteArgs(nil, "name", "", appIDs); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
