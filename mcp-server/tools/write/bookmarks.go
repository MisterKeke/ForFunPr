package writetools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterBookmarks(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name: "create_bookmark", Title: "Create bookmark",
		Description: "Save an HTTP or HTTPS URL in the local read-later collection without fetching it.",
		InputSchema: schemas.CreateBookmarkInputSchema,
		Annotations: tools.WriteAnnotations(false, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.CreateBookmarkInput) (*mcp.CallToolResult, schemas.Bookmark, error) {
		urlValue, err := schemas.RequiredString("url", input.URL)
		if err != nil {
			return nil, schemas.Bookmark{}, err
		}
		title, err := schemas.RequiredString("title", input.Title)
		if err != nil {
			return nil, schemas.Bookmark{}, err
		}
		args := []string{"bookmarks", "create", "--url", urlValue, "--title", title}
		args = tools.OptionalStringFlag(args, "--description", schemas.OptionalString(input.Description))
		for _, tag := range input.Tags {
			args = append(args, "--tag", tag)
		}
		return tools.Execute[schemas.Bookmark](ctx, runner, args, "Created a local bookmark.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "update_bookmark", Title: "Update bookmark",
		Description: "Update selected fields of a local bookmark while preserving unspecified fields.",
		InputSchema: schemas.UpdateBookmarkInputSchema,
		Annotations: tools.WriteAnnotations(true, false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.UpdateBookmarkInput) (*mcp.CallToolResult, schemas.Bookmark, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Bookmark{}, err
		}
		if input.Tags != nil && input.ClearTags {
			return nil, schemas.Bookmark{}, fmt.Errorf("tags and clear_tags cannot both be set")
		}
		if input.URL == nil && input.Title == nil && input.Description == nil && input.Tags == nil && !input.ClearTags {
			return nil, schemas.Bookmark{}, fmt.Errorf("at least one bookmark field is required")
		}
		args := []string{"bookmarks", "update", positiveInteger(id)}
		if input.URL != nil {
			args = append(args, "--url", *input.URL)
		}
		if input.Title != nil {
			args = append(args, "--title", *input.Title)
		}
		if input.Description != nil {
			args = append(args, "--description", *input.Description)
		}
		for _, tag := range input.Tags {
			args = append(args, "--tag", tag)
		}
		if input.ClearTags {
			args = append(args, "--clear-tags")
		}
		return tools.Execute[schemas.Bookmark](ctx, runner, args, "Updated the requested bookmark.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "set_bookmark_read", Title: "Set bookmark read state",
		Description: "Explicitly mark one local bookmark as read or unread.",
		InputSchema: schemas.BookmarkReadInputSchema,
		Annotations: tools.WriteAnnotations(false, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.BookmarkReadInput) (*mcp.CallToolResult, schemas.Bookmark, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.Bookmark{}, err
		}
		command := "mark-unread"
		if input.Read {
			command = "mark-read"
		}
		return tools.Execute[schemas.Bookmark](ctx, runner, []string{"bookmarks", command, positiveInteger(id)}, "Updated the bookmark read state.")
	})

	tools.AddTool(server, &mcp.Tool{
		Name: "delete_bookmark", Title: "Delete bookmark",
		Description: "Permanently delete one local bookmark.",
		InputSchema: schemas.BookmarkIDInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.BookmarkIDInput) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](ctx, runner, []string{"bookmarks", "delete", positiveInteger(id)}, "Deleted the requested bookmark.")
	})
}
