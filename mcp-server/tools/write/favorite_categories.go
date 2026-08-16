package writetools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

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

	tools.AddTool(server, &mcp.Tool{
		Name:        "rename_favorite_category",
		Title:       "Rename favorite category",
		Description: "Rename an existing local favorite category without changing its assignments.",
		InputSchema: schemas.RenameFavoriteCategoryInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.RenameFavoriteCategoryInput,
	) (*mcp.CallToolResult, schemas.FavoriteCategory, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.FavoriteCategory{}, err
		}
		name, err := schemas.RequiredString("name", input.Name)
		if err != nil {
			return nil, schemas.FavoriteCategory{}, err
		}
		return tools.Execute[schemas.FavoriteCategory](
			ctx,
			runner,
			[]string{
				"favorite-categories", "rename",
				"--id", positiveInteger(id),
				"--name", name,
			},
			"Renamed the requested favorite category.",
		)
	})
}
