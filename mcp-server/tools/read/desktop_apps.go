package readtools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterDesktopApps(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "list_desktop_apps", Title: "List desktop applications",
		Description: "List applications previously saved through the desktop picker. Filesystem paths are intentionally omitted.",
		InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.DesktopAppsOutput, error) {
		items, err := tools.Run[[]schemas.DesktopApp](ctx, runner, []string{"desktop-apps", "list"})
		return tools.Response(fmt.Sprintf("Listed %d saved desktop applications.", len(items)), schemas.DesktopAppsOutput{Applications: items}, err)
	})
}
