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
	) (*mcp.CallToolResult, schemas.FavoriteCategoryMutationOutput, error) {
		name, err := schemas.RequiredString("name", input.Name)
		if err != nil {
			return nil, schemas.FavoriteCategoryMutationOutput{}, err
		}
		source, err := schemas.FavoriteSource(input.Source, true)
		if err != nil {
			return nil, schemas.FavoriteCategoryMutationOutput{}, err
		}
		args := []string{
			"favorite-categories", "create", "--name", name, "--source", source,
		}
		if input.Color != "" {
			args = append(args, "--color", input.Color)
		}
		args = tools.OptionalIntFlag(args, "--display-order", input.DisplayOrder)
		return tools.Execute[schemas.FavoriteCategoryMutationOutput](
			ctx,
			runner,
			args,
			"Created the favorite category, or returned the existing category.",
		)
	})

	registerFavoriteCategoryUpdateTool(server, runner, "update_favorite_category")
	// Preserve the original MCP name while steering new clients to the wider
	// update contract.
	registerFavoriteCategoryUpdateTool(server, runner, "rename_favorite_category")

	tools.AddTool(server, &mcp.Tool{
		Name: "delete_favorite_category", Title: "Delete favorite category",
		Description: "Delete a category and either unassign its favorites or move them to a same-source category.",
		InputSchema: schemas.DeleteFavoriteCategoryInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.DeleteFavoriteCategoryInput) (*mcp.CallToolResult, schemas.FavoriteCategoryDeleteOutput, error) {
		args := []string{"favorite-categories", "delete", "--id", positiveInteger(input.ID), "--mode", input.Mode}
		if input.TargetCategoryID != nil {
			args = append(args, "--target-id", positiveInteger(*input.TargetCategoryID))
		}
		return tools.Execute[schemas.FavoriteCategoryDeleteOutput](ctx, runner, args, "Deleted the requested favorite category.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "reorder_favorite_categories", Title: "Reorder favorite categories",
		Description: "Set the display order for source-scoped favorite categories.",
		InputSchema: schemas.ReorderFavoriteCategoriesInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.ReorderFavoriteCategoriesInput) (*mcp.CallToolResult, schemas.FavoriteCategoriesOutput, error) {
		args := []string{"favorite-categories", "reorder", "--source", input.Source}
		for _, id := range input.CategoryIDs {
			args = append(args, "--id", positiveInteger(id))
		}
		return tools.Execute[schemas.FavoriteCategoriesOutput](ctx, runner, args, "Reordered favorite categories.")
	})
}

func registerFavoriteCategoryUpdateTool(server *mcp.Server, runner *tools.Runner, name string) {
	description := "Update an existing local favorite category's name, color, or display order without changing its assignments."
	if name == "rename_favorite_category" {
		description = "Compatibility alias for update_favorite_category."
	}
	tools.AddTool(server, &mcp.Tool{
		Name: name, Title: "Update favorite category", Description: description,
		InputSchema: schemas.RenameFavoriteCategoryInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.RenameFavoriteCategoryInput) (*mcp.CallToolResult, schemas.FavoriteCategoryMutationOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.FavoriteCategoryMutationOutput{}, err
		}
		categoryName, err := schemas.RequiredString("name", input.Name)
		if err != nil {
			return nil, schemas.FavoriteCategoryMutationOutput{}, err
		}
		args := []string{"favorite-categories", "update", "--id", positiveInteger(id), "--name", categoryName}
		if input.Color != nil {
			args = append(args, "--color", *input.Color)
		}
		args = tools.OptionalIntFlag(args, "--display-order", input.DisplayOrder)
		return tools.Execute[schemas.FavoriteCategoryMutationOutput](ctx, runner, args, "Updated the requested favorite category.")
	})
}
