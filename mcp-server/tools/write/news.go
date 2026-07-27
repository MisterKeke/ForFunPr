package writetools

import (
	"context"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterNews adds the explicit news refresh mutation tool.
func RegisterNews(server *mcp.Server, runner *tools.Runner) {
	registerNewsScan(
		server,
		runner,
		"refresh_news",
		"Refresh news",
		"Contact saved Telegram and YouTube channels and replace the stored dashboard news with only items newly discovered by this refresh. Use only when the user explicitly asks to refresh or fetch newer news. For show news, list news, or check news, use list_news instead.",
		"refresh",
	)
}

func registerNewsScan(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	action string,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.WriteAnnotations(true, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.NewsOutput, error) {
		return tools.Execute[schemas.NewsOutput](
			ctx,
			runner,
			[]string{"news", action},
			"Completed the requested live favorite-channel news scan.",
		)
	})
}
