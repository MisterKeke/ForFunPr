package readtools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterHealth adds the desktop API health tool.
func RegisterHealth(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "get_health",
		Title:       "Get Something health",
		Description: "Check whether the running Something desktop backend is ready.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.HealthOutput, error) {
		return tools.Execute[schemas.HealthOutput](
			ctx,
			runner,
			[]string{"health"},
			"Checked the Something desktop API health.",
		)
	})
}
