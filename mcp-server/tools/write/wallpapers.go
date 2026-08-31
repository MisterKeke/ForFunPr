package writetools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterWallpapers(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{Name: "select_wallpaper", Title: "Select wallpaper", Description: "Change the running desktop application's selected wallpaper.", InputSchema: schemas.WallpaperSelectionInputSchema, Annotations: tools.WriteAnnotations(true, true, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.WallpaperSelectionInput) (*mcp.CallToolResult, schemas.WallpaperSettings, error) {
		selection, err := schemas.RequiredString("selection", input.Selection)
		if err != nil {
			return nil, schemas.WallpaperSettings{}, err
		}
		return tools.Execute[schemas.WallpaperSettings](ctx, runner, []string{"wallpapers", "select", "--selection", selection}, "Selected the requested wallpaper.")
	})
	tools.AddTool(server, &mcp.Tool{Name: "delete_user_wallpaper", Title: "Delete user wallpaper", Description: "Permanently delete one imported wallpaper file and reset the selection when necessary.", InputSchema: schemas.UserWallpaperIDInputSchema, Annotations: tools.WriteAnnotations(true, true, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.UserWallpaperIDInput) (*mcp.CallToolResult, schemas.WallpaperSettings, error) {
		id, err := schemas.RequiredString("id", input.ID)
		if err != nil {
			return nil, schemas.WallpaperSettings{}, err
		}
		return tools.Execute[schemas.WallpaperSettings](ctx, runner, []string{"wallpapers", "delete", "--id", id}, "Deleted the imported wallpaper.")
	})
}
