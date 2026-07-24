package readtools

import (
	"context"
	"fmt"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPosts adds Telegram and YouTube post retrieval tools.
func RegisterPosts(server *mcp.Server, runner *tools.Runner) {
	registerChannelPosts(
		server,
		runner,
		"list_telegram_posts",
		"List Telegram posts",
		"List projected posts for one Telegram channel, optionally before a non-negative cursor.",
		"telegram",
	)
	registerChannelPosts(
		server,
		runner,
		"list_youtube_posts",
		"List YouTube posts",
		"List projected posts for one YouTube handle or channel ID, optionally before a non-negative cursor.",
		"youtube",
	)
	registerFavoritePosts(
		server,
		runner,
		"list_favorite_telegram_posts",
		"List favorite Telegram posts",
		"List projected posts from all saved Telegram channels.",
		"telegram",
	)
	registerFavoritePosts(
		server,
		runner,
		"list_favorite_youtube_posts",
		"List favorite YouTube posts",
		"List projected posts from all saved YouTube channels.",
		"youtube",
	)
}

func registerChannelPosts(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.ChannelPostsInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.ChannelPostsInput,
	) (*mcp.CallToolResult, schemas.PostsOutput, error) {
		channel, err := schemas.RequiredString("channel", input.Channel)
		if err != nil {
			return nil, schemas.PostsOutput{}, err
		}
		before, err := schemas.PaginationCursor(input.Before)
		if err != nil {
			return nil, schemas.PostsOutput{}, err
		}

		args := []string{"posts", source, "--channel", channel}
		args = tools.OptionalIntFlag(args, "--before", before)
		items, err := tools.Run[[]schemas.ChannelDate](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf("Listed %d projected %s posts.", len(items), source),
			schemas.PostsOutput{Posts: items},
			err,
		)
	})
}

func registerFavoritePosts(
	server *mcp.Server,
	runner *tools.Runner,
	name string,
	title string,
	description string,
	source string,
) {
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: schemas.FavoritePostsInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.FavoritePostsInput,
	) (*mcp.CallToolResult, schemas.PostsOutput, error) {
		before, err := schemas.PaginationCursor(input.Before)
		if err != nil {
			return nil, schemas.PostsOutput{}, err
		}

		args := []string{"posts", "favorites", source}
		args = tools.OptionalIntFlag(args, "--before", before)
		items, err := tools.Run[[]schemas.ChannelDate](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf(
				"Listed %d projected posts from %s favorites.",
				len(items),
				source,
			),
			schemas.PostsOutput{Posts: items},
			err,
		)
	})
}
