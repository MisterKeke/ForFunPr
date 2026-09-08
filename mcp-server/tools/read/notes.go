package readtools

import (
	"context"
	"fmt"
	"strconv"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterNotes(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "list_notes", Title: "List notes",
		Description: "Search local notes and filter active, archived, pinned, or unpinned notes.",
		InputSchema: schemas.NoteListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteListInput) (*mcp.CallToolResult, schemas.NotesOutput, error) {
		args := []string{"notes", "list"}
		args = tools.OptionalStringFlag(args, "--search", schemas.OptionalString(input.Query))
		args = tools.OptionalStringFlag(args, "--archive", schemas.OptionalString(input.Archive))
		if input.Pinned != nil {
			if *input.Pinned {
				args = append(args, "--pinned")
			} else {
				args = append(args, "--unpinned")
			}
		}
		args = tools.OptionalIntFlag(args, "--limit", input.Limit)
		args = tools.OptionalIntFlag(args, "--offset", input.Offset)
		output, err := tools.Run[schemas.NotesOutput](ctx, runner, args)
		return tools.Response(fmt.Sprintf("Listed %d of %d notes.", len(output.Notes), output.Total), output, err)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "get_note", Title: "Get note",
		Description: "Read one complete local note by ID.",
		InputSchema: schemas.NoteIDInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteIDInput) (*mcp.CallToolResult, schemas.Note, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Note{}, err
		}
		return tools.Execute[schemas.Note](ctx, runner, []string{"notes", "get", strconv.Itoa(id)}, "Loaded the requested note.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "list_note_topics", Title: "List note topics",
		Description: "List local note topic boards with bounded pagination.",
		InputSchema: schemas.NoteTopicListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicListInput) (*mcp.CallToolResult, schemas.NoteTopicsOutput, error) {
		args := []string{"notes", "topics", "list"}
		args = tools.OptionalIntFlag(args, "--limit", input.Limit)
		args = tools.OptionalIntFlag(args, "--offset", input.Offset)
		output, err := tools.Run[schemas.NoteTopicsOutput](ctx, runner, args)
		return tools.Response(fmt.Sprintf("Listed %d of %d note topics.", len(output.Items), output.Total), output, err)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "get_note_topic_board", Title: "Get note topic board",
		Description: "Read one topic with its note blocks, connections, and current revision.",
		InputSchema: schemas.NoteTopicIDInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicIDInput) (*mcp.CallToolResult, schemas.NoteTopicBoard, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.NoteTopicBoard{}, err
		}
		return tools.Execute[schemas.NoteTopicBoard](ctx, runner, []string{"notes", "topics", "get", strconv.Itoa(id)}, "Loaded the note topic board.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "search_note_topic_picker", Title: "Search topic note picker",
		Description: "Search active notes not already present on a topic board.",
		InputSchema: schemas.NoteTopicPickerInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicPickerInput) (*mcp.CallToolResult, schemas.NoteTopicPickerOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.NoteTopicPickerOutput{}, err
		}
		args := []string{"notes", "topics", "picker", strconv.Itoa(id)}
		args = tools.OptionalStringFlag(args, "--search", schemas.OptionalString(input.Query))
		args = tools.OptionalIntFlag(args, "--limit", input.Limit)
		args = tools.OptionalIntFlag(args, "--offset", input.Offset)
		output, err := tools.Run[schemas.NoteTopicPickerOutput](ctx, runner, args)
		return tools.Response(fmt.Sprintf("Found %d of %d eligible notes.", len(output.Items), output.Total), output, err)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "list_note_tasks", Title: "List note tasks",
		Description: "List tasks linked to one note.",
		InputSchema: schemas.NoteTodoInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTodoInput) (*mcp.CallToolResult, schemas.NoteTasksOutput, error) {
		noteID, err := schemas.PositiveID("note_id", input.NoteID)
		if err != nil {
			return nil, schemas.NoteTasksOutput{}, err
		}
		items, err := tools.Run[[]schemas.Task](ctx, runner, []string{"notes", "tasks", "list", strconv.Itoa(noteID)})
		return tools.Response("Loaded tasks linked to the note.", schemas.NoteTasksOutput{Items: items}, err)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "list_task_notes", Title: "List task notes",
		Description: "List notes linked to one task.",
		InputSchema: schemas.TaskNotesInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.TaskNotesInput) (*mcp.CallToolResult, schemas.TaskNotesOutput, error) {
		taskID, err := schemas.PositiveID("task_id", input.TaskID)
		if err != nil {
			return nil, schemas.TaskNotesOutput{}, err
		}
		items, err := tools.Run[[]schemas.NoteSummary](ctx, runner, []string{"notes", "tasks", "notes", strconv.Itoa(taskID)})
		return tools.Response("Loaded notes linked to the task.", schemas.TaskNotesOutput{Items: items}, err)
	})
}
