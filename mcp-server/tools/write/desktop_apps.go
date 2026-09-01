package writetools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterDesktopApps(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "rename_desktop_app", Title: "Rename desktop application",
		Description: "Replace the display name of an application previously saved through the desktop UI.",
		InputSchema: schemas.RenameDesktopAppInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.RenameDesktopAppInput) (*mcp.CallToolResult, schemas.DesktopApp, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.DesktopApp{}, err
		}
		name, err := schemas.RequiredString("display_name", input.DisplayName)
		if err != nil {
			return nil, schemas.DesktopApp{}, err
		}
		return tools.Execute[schemas.DesktopApp](ctx, runner, []string{"desktop-apps", "rename", "--id", positiveInteger(id), "--name", name}, "Renamed the saved desktop application.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "launch_desktop_app",
		Title:       "Launch desktop application",
		Description: "Launch one application previously saved through the desktop UI. This causes an external local process to start; call it only after the user explicitly requests that exact application.",
		InputSchema: schemas.LaunchDesktopAppInputSchema,
		Annotations: tools.WriteAnnotations(true, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.LaunchDesktopAppInput,
	) (*mcp.CallToolResult, schemas.DesktopAppLaunchResult, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.DesktopAppLaunchResult{}, err
		}
		if !input.Confirm {
			return nil, schemas.DesktopAppLaunchResult{}, fmt.Errorf("confirm must be true to launch a desktop application")
		}
		return tools.Execute[schemas.DesktopAppLaunchResult](
			ctx,
			runner,
			[]string{"desktop-apps", "launch", "--id", positiveInteger(id), "--confirm"},
			"Launched the requested saved desktop application.",
		)
	})
}
