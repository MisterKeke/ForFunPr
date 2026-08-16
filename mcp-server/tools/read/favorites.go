package readtools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterFavorites adds local Telegram and YouTube favorite list tools.
func RegisterFavorites(server *mcp.Server, runner *tools.Runner) {
	registerFavorites(
		server,
		runner,
		"list_telegram_favorites",
		"List Telegram favorites",
		"List saved Telegram channel usernames.",
		"telegram",
		false,
	)
	registerFavorites(
		server,
		runner,
		"list_categorized_telegram_favorites",
		"List categorized Telegram favorites",
		"List saved Telegram channels with category assignments.",
		"telegram",
		true,
	)
	registerFavorites(
		server,
		runner,
		"list_youtube_favorites",
		"List YouTube favorites",
		"List saved YouTube channel IDs.",
		"youtube",
		false,
	)
	registerFavorites(
		server,
		runner,
		"list_categorized_youtube_favorites",
		"List categorized YouTube favorites",
		"List saved YouTube channels with category assignments.",
		"youtube",
		true,
	)
}

func registerFavorites(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
	categorized bool,
) {
	tool := &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}

	if categorized {
		tools.AddTool(server, tool, func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			_ schemas.EmptyInput,
		) (*mcp.CallToolResult, schemas.CategorizedFavoritesOutput, error) {
			return tools.Execute[schemas.CategorizedFavoritesOutput](
				ctx,
				runner,
				[]string{"favorites", source, "list-categories"},
				"Listed saved channels and their category assignments.",
			)
		})
		return
	}

	tools.AddTool(server, tool, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.FavoritesOutput, error) {
		return tools.Execute[schemas.FavoritesOutput](
			ctx,
			runner,
			[]string{"favorites", source, "list"},
			"Listed saved favorite channels.",
		)
	})
}
