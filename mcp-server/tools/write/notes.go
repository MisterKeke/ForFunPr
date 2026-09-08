package writetools

import (
	"context"
	"encoding/json"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterNotes(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "create_note", Title: "Create note",
		Description: "Create a new local plain-text note.",
		InputSchema: schemas.CreateNoteInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.CreateNoteInput) (*mcp.CallToolResult, schemas.Note, error) {
		args := []string{"notes", "create"}
		if input.Title != "" {
			args = append(args, "--title", input.Title)
		}
		if input.Body != "" {
			args = append(args, "--body", input.Body)
		}
		if input.Pinned {
			args = append(args, "--pin")
		}
		return tools.Execute[schemas.Note](ctx, runner, args, "Created a local note.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "update_note", Title: "Update note",
		Description: "Update the title or body of a local note while preserving unspecified fields.",
		InputSchema: schemas.UpdateNoteInputSchema,
		Annotations: tools.WriteAnnotations(true, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.UpdateNoteInput) (*mcp.CallToolResult, schemas.Note, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Note{}, err
		}
		if input.Title == nil && input.Body == nil {
			return nil, schemas.Note{}, fmt.Errorf("title or body is required")
		}
		args := []string{"notes", "update", positiveInteger(id)}
		if input.Title != nil {
			args = append(args, "--title", *input.Title)
		}
		if input.Body != nil {
			args = append(args, "--body", *input.Body)
		}
		return tools.Execute[schemas.Note](ctx, runner, args, "Updated the requested note.")
	})

	registerNoteStateTool(server, runner, "set_note_pinned", "Set note pinned state", "pin", "unpin")
	registerNoteStateTool(server, runner, "set_note_archived", "Set note archived state", "archive", "restore")

	tools.AddTool(server, &mcp.Tool{
		Name: "delete_note", Title: "Delete note",
		Description: "Permanently delete one local note.",
		InputSchema: schemas.NoteIDInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteIDInput) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](ctx, runner, []string{"notes", "delete", positiveInteger(id)}, "Deleted the requested note.")
	})

	registerNoteTopicWriteTools(server, runner)
}

func registerNoteTopicWriteTools(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "create_note_topic", Title: "Create note topic",
		Description: "Create a local note topic board.", InputSchema: schemas.NoteTopicWriteInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicWriteInput) (*mcp.CallToolResult, schemas.NoteTopic, error) {
		return tools.Execute[schemas.NoteTopic](ctx, runner, []string{"notes", "topics", "create", "--title", input.Title}, "Created the note topic.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "update_note_topic", Title: "Update note topic",
		Description: "Rename a topic with optional optimistic revision protection.", InputSchema: schemas.UpdateNoteTopicInputSchema,
		Annotations: tools.WriteAnnotations(true, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicWriteInput) (*mcp.CallToolResult, schemas.NoteTopic, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.NoteTopic{}, err
		}
		args := []string{"notes", "topics", "update", positiveInteger(id), "--title", input.Title}
		args = tools.OptionalIntFlag(args, "--expected-revision", input.ExpectedRevision)
		return tools.Execute[schemas.NoteTopic](ctx, runner, args, "Updated the note topic.")
	})

	registerNoteTopicDeleteTool(server, runner, "delete_note_topic", "Delete note topic", "delete")
	registerNoteTopicDeleteTool(server, runner, "delete_note_topic_block", "Delete note topic block", "delete-block")
	registerNoteTopicDeleteTool(server, runner, "delete_note_topic_connection", "Delete note topic connection", "disconnect")

	tools.AddTool(server, &mcp.Tool{
		Name: "add_note_topic_block", Title: "Add note topic block",
		Description: "Add a note to a topic board.", InputSchema: schemas.NoteTopicBlockInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicBlockInput) (*mcp.CallToolResult, schemas.NoteTopicBlock, error) {
		topicID, err := schemas.PositiveID("topic_id", input.TopicID)
		if err != nil {
			return nil, schemas.NoteTopicBlock{}, err
		}
		noteID, err := schemas.PositiveID("note_id", input.NoteID)
		if err != nil {
			return nil, schemas.NoteTopicBlock{}, err
		}
		args := []string{"notes", "topics", "add-block", positiveInteger(topicID), "--note-id", positiveInteger(noteID), "--x", fmt.Sprint(input.PositionX), "--y", fmt.Sprint(input.PositionY)}
		args = tools.OptionalIntFlag(args, "--expected-revision", input.ExpectedRevision)
		return tools.Execute[schemas.NoteTopicBlock](ctx, runner, args, "Added the note to the topic.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "move_note_topic_blocks", Title: "Move note topic blocks",
		Description: "Atomically update one or more board block positions.", InputSchema: schemas.NoteTopicPositionsInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicPositionsInput) (*mcp.CallToolResult, schemas.NoteTopicMutationOutput, error) {
		topicID, err := schemas.PositiveID("topic_id", input.TopicID)
		if err != nil {
			return nil, schemas.NoteTopicMutationOutput{}, err
		}
		positions, err := json.Marshal(input.Positions)
		if err != nil {
			return nil, schemas.NoteTopicMutationOutput{}, fmt.Errorf("encode positions: %w", err)
		}
		args := []string{"notes", "topics", "move-blocks", positiveInteger(topicID), "--positions", string(positions)}
		args = tools.OptionalIntFlag(args, "--expected-revision", input.ExpectedRevision)
		return tools.Execute[schemas.NoteTopicMutationOutput](ctx, runner, args, "Moved the topic blocks.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "create_note_topic_connection", Title: "Create note topic connection",
		Description: "Connect two blocks on the same topic board.", InputSchema: schemas.NoteTopicConnectionInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicConnectionInput) (*mcp.CallToolResult, schemas.NoteTopicConnection, error) {
		topicID, err := schemas.PositiveID("topic_id", input.TopicID)
		if err != nil {
			return nil, schemas.NoteTopicConnection{}, err
		}
		args := []string{"notes", "topics", "connect", positiveInteger(topicID), "--from-block-id", positiveInteger(input.FromBlockID), "--to-block-id", positiveInteger(input.ToBlockID)}
		args = tools.OptionalStringFlag(args, "--relation", schemas.OptionalString(input.RelationType))
		args = tools.OptionalIntFlag(args, "--expected-revision", input.ExpectedRevision)
		return tools.Execute[schemas.NoteTopicConnection](ctx, runner, args, "Connected the topic blocks.")
	})

	for _, definition := range []struct{ Name, Title, Command string }{{"link_note_task", "Link note task", "link"}, {"unlink_note_task", "Unlink note task", "unlink"}} {
		definition := definition
		tools.AddTool(server, &mcp.Tool{Name: definition.Name, Title: definition.Title, Description: definition.Title + ".", InputSchema: schemas.NoteTodoWriteInputSchema, Annotations: tools.WriteAnnotations(false, false, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTodoInput) (*mcp.CallToolResult, schemas.NoteTodoMutationOutput, error) {
			noteID, err := schemas.PositiveID("note_id", input.NoteID)
			if err != nil {
				return nil, schemas.NoteTodoMutationOutput{}, err
			}
			todoID, err := schemas.PositiveID("todo_id", input.TodoID)
			if err != nil {
				return nil, schemas.NoteTodoMutationOutput{}, err
			}
			return tools.Execute[schemas.NoteTodoMutationOutput](ctx, runner, []string{"notes", "tasks", definition.Command, positiveInteger(noteID), positiveInteger(todoID)}, definition.Title+" completed.")
		})
	}
}

func registerNoteTopicDeleteTool(server *mcp.Server, runner *tools.Runner, name, title, command string) {
	tools.AddTool(server, &mcp.Tool{Name: name, Title: title, Description: title + ".", InputSchema: schemas.NoteTopicIDInputSchema, Annotations: tools.WriteAnnotations(true, true, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteTopicIDInput) (*mcp.CallToolResult, schemas.NoteTopicMutationOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.NoteTopicMutationOutput{}, err
		}
		args := []string{"notes", "topics", command, positiveInteger(id)}
		args = tools.OptionalIntFlag(args, "--expected-revision", input.ExpectedRevision)
		return tools.Execute[schemas.NoteTopicMutationOutput](ctx, runner, args, title+" completed.")
	})
}

func registerNoteStateTool(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	trueCommand string,
	falseCommand string,
) {
	tools.AddTool(server, &mcp.Tool{
		Name: name, Title: title,
		Description: "Set an explicit state on one local note.",
		InputSchema: schemas.NoteStateInputSchema,
		Annotations: tools.WriteAnnotations(false, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.NoteStateInput) (*mcp.CallToolResult, schemas.Note, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Note{}, err
		}
		command := falseCommand
		if input.Value {
			command = trueCommand
		}
		return tools.Execute[schemas.Note](ctx, runner, []string{"notes", command, positiveInteger(id)}, "Updated the requested note state.")
	})
}
