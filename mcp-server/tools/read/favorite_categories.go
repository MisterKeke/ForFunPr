package readtools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterFavoriteCategories adds the source-filtered category list tool.
func RegisterFavoriteCategories(
	server *mcp.Server,
	runner *tools.Runner,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "list_favorite_categories",
		Title:       "List favorite categories",
		Description: "List local favorite categories for Telegram or YouTube; defaults to Telegram.",
		InputSchema: schemas.FavoriteCategoryListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.FavoriteCategoryListInput,
	) (*mcp.CallToolResult, schemas.FavoriteCategoriesOutput, error) {
		source, err := schemas.FavoriteSource(input.Source, false)
		if err != nil {
			return nil, schemas.FavoriteCategoriesOutput{}, err
		}

		args := []string{"favorite-categories", "list"}
		args = tools.OptionalStringFlag(args, "--source", source)
		return tools.Execute[schemas.FavoriteCategoriesOutput](
			ctx,
			runner,
			args,
			"Listed favorite categories for the selected source.",
		)
	})
}
