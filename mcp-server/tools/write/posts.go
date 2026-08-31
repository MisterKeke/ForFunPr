package writetools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPosts adds explicit provider refresh tools. Ordinary post-listing
// requests remain read tools and may use the existing in-memory cache.
func RegisterPosts(server *mcp.Server, runner *tools.Runner) {
	registerRefreshChannelPosts(server, runner, "telegram")
	registerRefreshChannelPosts(server, runner, "youtube")
}

func registerRefreshChannelPosts(server *mcp.Server, runner *tools.Runner, source string) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "refresh_" + source + "_posts",
		Title:       "Refresh " + source + " posts",
		Description: "Bypass the in-memory cache and retrieve the latest posts for one " + source + " channel. Use only when the user explicitly requests fresh provider data.",
		InputSchema: schemas.RefreshChannelPostsInputSchema,
		Annotations: tools.WriteAnnotations(false, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.ChannelPostsInput,
	) (*mcp.CallToolResult, schemas.PostsOutput, error) {
		channel, err := schemas.RequiredString("channel", input.Channel)
		if err != nil {
			return nil, schemas.PostsOutput{}, err
		}
		items, err := tools.Run[[]schemas.Post](
			ctx,
			runner,
			[]string{"posts", "refresh", source, "--channel", channel},
		)
		return tools.Response(
			fmt.Sprintf("Refreshed and listed %d %s posts.", len(items), source),
			schemas.PostsOutput{Posts: items},
			err,
		)
	})
}
