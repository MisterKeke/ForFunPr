package writetools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterFavorites adds Telegram and YouTube favorite mutations.
func RegisterFavorites(server *mcp.Server, runner *tools.Runner) {
	registerAddFavorite(
		server,
		runner,
		"add_telegram_favorite",
		"Add Telegram favorite",
		"Add a Telegram channel to local favorites.",
		"telegram",
		false,
	)
	registerRemoveFavorite(
		server,
		runner,
		"remove_telegram_favorite",
		"Remove Telegram favorite",
		"Remove a Telegram channel from local favorites.",
		"telegram",
		false,
	)
	registerAssignFavoriteCategory(
		server,
		runner,
		"assign_telegram_favorite_category",
		"Assign Telegram favorite category",
		"Overwrite the category assigned to a saved Telegram channel.",
		"telegram",
		false,
	)
	registerAddFavorite(
		server,
		runner,
		"add_youtube_favorite",
		"Add YouTube favorite",
		"Resolve when needed and add a YouTube channel to local favorites.",
		"youtube",
		true,
	)
	registerRemoveFavorite(
		server,
		runner,
		"remove_youtube_favorite",
		"Remove YouTube favorite",
		"Resolve when needed and remove a YouTube channel from local favorites.",
		"youtube",
		true,
	)
	registerAssignFavoriteCategory(
		server,
		runner,
		"assign_youtube_favorite_category",
		"Assign YouTube favorite category",
		"Resolve when needed and overwrite the category assigned to a saved YouTube channel.",
		"youtube",
		true,
	)
}

func registerAddFavorite(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
	openWorld bool,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.ChannelInputSchema,
		Annotations: tools.WriteAnnotations(false, true, openWorld),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.ChannelInput,
	) (*mcp.CallToolResult, schemas.FavoritesOutput, error) {
		channel, err := schemas.RequiredString("channel", input.Channel)
		if err != nil {
			return nil, schemas.FavoritesOutput{}, err
		}
		return tools.Execute[schemas.FavoritesOutput](
			ctx,
			runner,
			[]string{
				"favorites", source, "add",
				"--channel", channel,
			},
			"Added the channel to favorites, or confirmed it was already saved.",
		)
	})
}

func registerRemoveFavorite(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
	openWorld bool,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.ChannelInputSchema,
		Annotations: tools.WriteAnnotations(true, true, openWorld),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.ChannelInput,
	) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		channel, err := schemas.RequiredString("channel", input.Channel)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](
			ctx,
			runner,
			[]string{
				"favorites", source, "remove",
				"--channel", channel,
			},
			"Removed the requested channel from favorites.",
		)
	})
}

func registerAssignFavoriteCategory(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
	openWorld bool,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.ChannelCategoryInputSchema,
		Annotations: tools.WriteAnnotations(true, true, openWorld),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.ChannelCategoryInput,
	) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		channel, err := schemas.RequiredString("channel", input.Channel)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		categoryID, err := schemas.PositiveID(
			"category_id",
			input.CategoryID,
		)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](
			ctx,
			runner,
			[]string{
				"favorites", source, "assign-category",
				"--channel", channel,
				"--category-id", positiveInteger(categoryID),
			},
			"Assigned the requested category to the favorite channel.",
		)
	})
}
