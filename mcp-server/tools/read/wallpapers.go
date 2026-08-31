package readtools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterWallpapers(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{Name: "get_wallpaper_settings", Title: "Get wallpaper settings", Description: "Read the selected wallpaper and available built-in and user selection IDs.", InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false)}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.WallpaperSettings, error) {
		return tools.Execute[schemas.WallpaperSettings](ctx, runner, []string{"wallpapers", "settings"}, "Loaded wallpaper settings.")
	})
}
