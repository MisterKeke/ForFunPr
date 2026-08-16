package writetools

import (
	"context"
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
