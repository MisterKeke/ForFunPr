package writetools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSetups(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "create_setup", Title: "Create application setup",
		Description: "Create a saved group of desktop applications. Icon upload remains desktop-only.",
		InputSchema: schemas.CreateSetupInputSchema, Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.CreateSetupInput) (*mcp.CallToolResult, schemas.Setup, error) {
		args, err := setupWriteArgs([]string{"setups", "create"}, input.Name, input.Description, input.AppIDs)
		if err != nil {
			return nil, schemas.Setup{}, err
		}
		return tools.Execute[schemas.Setup](ctx, runner, args, "Created the application setup.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "start_setup",
		Title:       "Start application setup",
		Description: "Launch every saved desktop application in one setup. This can start multiple external local processes; call it only after the user explicitly requests that exact setup.",
		InputSchema: schemas.StartSetupInputSchema,
		Annotations: tools.WriteAnnotations(true, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.StartSetupInput,
	) (*mcp.CallToolResult, schemas.SetupStartResult, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.SetupStartResult{}, err
		}
		if !input.Confirm {
			return nil, schemas.SetupStartResult{}, fmt.Errorf("confirm must be true to start an application setup")
		}
		return tools.Execute[schemas.SetupStartResult](
			ctx,
			runner,
			[]string{"setups", "start", "--id", positiveInteger(id), "--confirm"},
			"Attempted to launch every application in the requested setup.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "update_setup", Title: "Update application setup",
		Description: "Replace a setup's name, description, and ordered application IDs; optionally remove its icon.",
		InputSchema: schemas.UpdateSetupInputSchema, Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.UpdateSetupInput) (*mcp.CallToolResult, schemas.Setup, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Setup{}, err
		}
		args, err := setupWriteArgs([]string{"setups", "update", "--id", positiveInteger(id)}, input.Name, input.Description, input.AppIDs)
		if err != nil {
			return nil, schemas.Setup{}, err
		}
		if input.RemoveIcon {
			args = append(args, "--remove-icon")
		}
		return tools.Execute[schemas.Setup](ctx, runner, args, "Updated the application setup.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "delete_setup", Title: "Delete application setup",
		Description: "Permanently delete one setup without deleting its applications.",
		InputSchema: schemas.SetupIDInputSchema, Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.SetupIDInput) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](ctx, runner, []string{"setups", "delete", "--id", positiveInteger(id)}, "Deleted the application setup.")
	})
}

func setupWriteArgs(args []string, nameValue string, descriptionValue string, appIDs []int) ([]string, error) {
	name, err := schemas.RequiredString("name", nameValue)
	if err != nil {
		return nil, err
	}
	if len(appIDs) == 0 {
		return nil, fmt.Errorf("app_ids must contain at least one application ID")
	}
	args = append(args, "--name", name)
	args = tools.OptionalStringFlag(args, "--description", schemas.OptionalString(descriptionValue))
	seen := make(map[int]struct{}, len(appIDs))
	for _, value := range appIDs {
		id, err := schemas.PositiveID("each app_id", value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("app_ids cannot contain duplicates")
		}
		seen[id] = struct{}{}
		args = append(args, "--app-id", positiveInteger(id))
	}
	return args, nil
}
