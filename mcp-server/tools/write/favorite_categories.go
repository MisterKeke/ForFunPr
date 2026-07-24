package writetools

import (
	"context"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterFavoriteCategories adds the local category creation tool.
func RegisterFavoriteCategories(
	server *mcp.Server,
	runner *tools.Runner,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "create_favorite_category",
		Title:       "Create favorite category",
		Description: "Create a local favorite category or return the existing source-scoped category with the same name.",
		InputSchema: schemas.CreateFavoriteCategoryInputSchema,
		Annotations: tools.WriteAnnotations(false, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CreateFavoriteCategoryInput,
	) (*mcp.CallToolResult, schemas.FavoriteCategory, error) {
		name, err := schemas.RequiredString("name", input.Name)
		if err != nil {
			return nil, schemas.FavoriteCategory{}, err
		}
		source, err := schemas.FavoriteSource(input.Source, true)
		if err != nil {
			return nil, schemas.FavoriteCategory{}, err
		}
		return tools.Execute[schemas.FavoriteCategory](
			ctx,
			runner,
			[]string{
				"favorite-categories", "create",
				"--name", name,
				"--source", source,
			},
			"Created the favorite category, or returned the existing category.",
		)
	})
}
