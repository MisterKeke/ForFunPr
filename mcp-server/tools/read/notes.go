package readtools

import (
	"context"
	"fmt"
	"strconv"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

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
}
