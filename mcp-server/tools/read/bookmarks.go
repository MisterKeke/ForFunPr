package readtools

import (
	"context"
	"fmt"
	"strconv"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterBookmarks(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "list_bookmarks", Title: "List bookmarks",
		Description: "Search local read-later bookmarks and filter them by read state and exact tags.",
		InputSchema: schemas.BookmarkListInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.BookmarkListInput) (*mcp.CallToolResult, schemas.BookmarksOutput, error) {
		args := []string{"bookmarks", "list"}
		args = tools.OptionalStringFlag(args, "--search", schemas.OptionalString(input.Query))
		args = tools.OptionalStringFlag(args, "--status", schemas.OptionalString(input.Status))
		for _, tag := range input.Tags {
			args = append(args, "--tag", tag)
		}
		args = tools.OptionalIntFlag(args, "--limit", input.Limit)
		args = tools.OptionalIntFlag(args, "--offset", input.Offset)
		output, err := tools.Run[schemas.BookmarksOutput](ctx, runner, args)
		return tools.Response(fmt.Sprintf("Listed %d of %d bookmarks.", len(output.Bookmarks), output.Total), output, err)
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "get_bookmark", Title: "Get bookmark",
		Description: "Read one complete local bookmark by ID.",
		InputSchema: schemas.BookmarkIDInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.BookmarkIDInput) (*mcp.CallToolResult, schemas.Bookmark, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Bookmark{}, err
		}
		return tools.Execute[schemas.Bookmark](ctx, runner, []string{"bookmarks", "get", strconv.Itoa(id)}, "Loaded the requested bookmark.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "list_bookmark_tags", Title: "List bookmark tags",
		Description: "List tags currently assigned to local bookmarks.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.BookmarkTagsOutput, error) {
		tags, err := tools.Run[[]string](ctx, runner, []string{"bookmarks", "tags"})
		return tools.Response(fmt.Sprintf("Listed %d bookmark tags.", len(tags)), schemas.BookmarkTagsOutput{Tags: tags}, err)
	})
}
