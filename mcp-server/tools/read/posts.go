package readtools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPosts adds Telegram and YouTube post retrieval tools.
func RegisterPosts(server *mcp.Server, runner *tools.Runner) {
	registerChannelPosts(
		server,
		runner,
		"list_telegram_posts",
		"List Telegram posts",
		"List full posts for one Telegram channel, optionally before a non-negative cursor.",
		"telegram",
	)
	registerChannelPosts(
		server,
		runner,
		"list_youtube_posts",
		"List YouTube posts",
		"List the latest posts for one YouTube handle or channel ID. The provider feed does not support pagination.",
		"youtube",
	)
	registerFavoritePosts(
		server,
		runner,
		"list_favorite_telegram_posts",
		"List favorite Telegram posts",
		"List full posts from all saved Telegram channels.",
		"telegram",
	)
	registerFavoritePosts(
		server,
		runner,
		"list_favorite_youtube_posts",
		"List favorite YouTube posts",
		"List full posts from all saved YouTube channels.",
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
	inputSchema := schemas.ChannelPostsInputSchema
	if source == "youtube" {
		inputSchema = schemas.YouTubeChannelPostsInputSchema
	}
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: inputSchema,
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
		var before *int
		if source == "telegram" {
			before, err = schemas.PaginationCursor(input.Before)
			if err != nil {
				return nil, schemas.PostsOutput{}, err
			}
		}

		args := []string{"posts", source, "--channel", channel}
		args = tools.OptionalIntFlag(args, "--before", before)
		items, err := tools.Run[[]schemas.Post](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf("Listed %d %s posts.", len(items), source),
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
	inputSchema := schemas.FavoritePostsInputSchema
	if source == "youtube" {
		inputSchema = schemas.YouTubeFavoritePostsInputSchema
	}
	tools.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: inputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.FavoritePostsInput,
	) (*mcp.CallToolResult, schemas.PostsOutput, error) {
		var before *int
		var err error
		if source == "telegram" {
			before, err = schemas.PaginationCursor(input.Before)
			if err != nil {
				return nil, schemas.PostsOutput{}, err
			}
		}

		args := []string{"posts", "favorites", source}
		args = tools.OptionalIntFlag(args, "--before", before)
		items, err := tools.Run[[]schemas.Post](ctx, runner, args)
		return tools.Response(
			fmt.Sprintf(
				"Listed %d posts from %s favorites.",
				len(items),
				source,
			),
			schemas.PostsOutput{Posts: items},
			err,
		)
	})
}
