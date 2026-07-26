package writetools

import (
	"context"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterNews adds the news scan and refresh mutation tools.
func RegisterNews(server *mcp.Server, runner *tools.Runner) {
	registerNewsScan(
		server,
		runner,
		"scan_initial_news",
		"Scan initial news",
		"Run the initial live scan of saved Telegram and YouTube channels and return full source-specific posts.",
		"initial",
	)
	registerNewsScan(
		server,
		runner,
		"refresh_news",
		"Refresh news",
		"Refresh live news for saved Telegram and YouTube channels and return full source-specific posts and errors.",
		"refresh",
	)
	registerNewsScan(
		server,
		runner,
		"scan_news_since_last_open",
		"Scan news since last open",
		"Scan saved channels for live news published since the previous app open and return full source-specific posts.",
		"since-last-open",
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
