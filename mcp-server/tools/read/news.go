package readtools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterNews adds all GET-backed news CLI commands.
func RegisterNews(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "list_news",
		Title:       "List latest news",
		Description: "Read the latest stored favorite-channel news scan without contacting providers or advancing refresh state. This matches what the dashboard currently shows. Always prefer this tool when the user asks to show, read, or list news.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.NewsOutput, error) {
		return tools.Execute[schemas.NewsOutput](
			ctx,
			runner,
			[]string{"news", "list"},
			"Listed the latest stored favorite-channel news scan.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "get_news_state",
		Title:       "Get news state",
		Description: "Read local application-open and news-refresh timestamps and windows.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.NewsStateOutput, error) {
		return tools.Execute[schemas.NewsStateOutput](
			ctx,
			runner,
			[]string{"news", "state"},
			"Loaded the local news refresh state.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "get_news_windows",
		Title:       "Get news windows",
		Description: "Read the local publication windows used for favorite-channel scans.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.UpdateWindowsOutput, error) {
		return tools.Execute[schemas.UpdateWindowsOutput](
			ctx,
			runner,
			[]string{"news", "windows"},
			"Loaded the local news update windows.",
		)
	})
}
