package readtools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSetups(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "list_setups", Title: "List application setups",
		Description: "List saved groups of desktop application IDs without launching them.",
		InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.SetupsOutput, error) {
		items, err := tools.Run[[]schemas.Setup](ctx, runner, []string{"setups", "list"})
		return tools.Response(fmt.Sprintf("Listed %d application setups.", len(items)), schemas.SetupsOutput{Setups: items}, err)
	})
}
