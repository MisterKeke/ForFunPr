package cmd

import (
	"fmt"
	"strings"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newBookmarksCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "bookmarks", Short: "Manage read-later bookmarks", Args: cobra.NoArgs}
	command.AddCommand(
		newBookmarkListCommand(dependencies),
		newBookmarkGetCommand(dependencies),
		newBookmarkCreateCommand(dependencies),
		newBookmarkUpdateCommand(dependencies),
		newBookmarkReadCommand(dependencies, "mark-read", "Mark a bookmark as read", true),
		newBookmarkReadCommand(dependencies, "mark-unread", "Mark a bookmark as unread", false),
		newBookmarkDeleteCommand(dependencies),
		newBookmarkTagsCommand(dependencies),
	)
	return command
}

func newBookmarkListCommand(dependencies commandDependencies) *cobra.Command {
	var filter apiclient.BookmarkFilter
	filter.Status = "all"
	command := &cobra.Command{
		Use: "list", Short: "List bookmarks", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.ListBookmarks(command.Context(), filter)
			if err != nil {
				return fmt.Errorf("list bookmarks: %w", err)
			}
			if strings.EqualFold(dependencies.outputFormat(), "json") {
				return dependencies.writeValue(command, result)
			}
			return dependencies.writeValue(command, result.Bookmarks)
		},
	}
	command.Flags().StringVar(&filter.Query, "search", "", "search bookmark titles, URLs, descriptions, and tags")
	command.Flags().StringVar(&filter.Status, "status", "all", "all, unread, or read")
	command.Flags().StringSliceVar(&filter.Tags, "tag", nil, "required bookmark tag; may be repeated")
	command.Flags().IntVar(&filter.Limit, "limit", 50, "maximum bookmarks to return")
	command.Flags().IntVar(&filter.Offset, "offset", 0, "bookmarks to skip")
	return command
}

func newBookmarkGetCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "get [BOOKMARK_ID]", Short: "Get a bookmark", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Bookmark ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			bookmark, err := client.GetBookmark(command.Context(), id)
			if err != nil {
				return fmt.Errorf("get bookmark: %w", err)
			}
			return dependencies.writeValue(command, bookmark)
		},
	}
}

func newBookmarkCreateCommand(dependencies commandDependencies) *cobra.Command {
	var request apiclient.BookmarkWriteRequest
	command := &cobra.Command{
		Use: "create", Short: "Create a bookmark", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("URL", &request.URL); err != nil {
				return err
			}
			if err := prompt.required("Title", &request.Title); err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			bookmark, err := client.CreateBookmark(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create bookmark: %w", err)
			}
			return dependencies.writeValue(command, bookmark)
		},
	}
	command.Flags().StringVar(&request.URL, "url", "", "HTTP or HTTPS URL")
	command.Flags().StringVar(&request.Title, "title", "", "bookmark title")
	command.Flags().StringVar(&request.Description, "description", "", "bookmark description")
	command.Flags().StringSliceVar(&request.Tags, "tag", nil, "bookmark tag; may be repeated")
	return command
}

func newBookmarkUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var urlValue string
	var title string
	var description string
	var tags []string
	var clearTags bool
	command := &cobra.Command{
		Use: "update [BOOKMARK_ID]", Short: "Update a bookmark", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Bookmark ID", arguments[0])
			if err != nil {
				return err
			}
			if clearTags && command.Flags().Changed("tag") {
				return fmt.Errorf("--tag and --clear-tags cannot be used together")
			}
			if !command.Flags().Changed("url") && !command.Flags().Changed("title") &&
				!command.Flags().Changed("description") && !command.Flags().Changed("tag") && !clearTags {
				return fmt.Errorf("set at least one bookmark field")
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			current, err := client.GetBookmark(command.Context(), id)
			if err != nil {
				return fmt.Errorf("load bookmark before update: %w", err)
			}
			request := apiclient.BookmarkWriteRequest{
				URL: current.URL, Title: current.Title, Description: current.Description,
				Tags: current.Tags, ExpectedRevision: current.Revision,
			}
			if command.Flags().Changed("url") {
				request.URL = urlValue
			}
			if command.Flags().Changed("title") {
				request.Title = title
			}
			if command.Flags().Changed("description") {
				request.Description = description
			}
			if command.Flags().Changed("tag") {
				request.Tags = tags
			} else if clearTags {
				request.Tags = []string{}
			}
			bookmark, err := client.UpdateBookmark(command.Context(), id, request)
			if err != nil {
				return fmt.Errorf("update bookmark: %w", err)
			}
			return dependencies.writeValue(command, bookmark)
		},
	}
	command.Flags().StringVar(&urlValue, "url", "", "replacement URL")
	command.Flags().StringVar(&title, "title", "", "replacement title")
	command.Flags().StringVar(&description, "description", "", "replacement description")
	command.Flags().StringSliceVar(&tags, "tag", nil, "replacement bookmark tag; may be repeated")
	command.Flags().BoolVar(&clearTags, "clear-tags", false, "remove every bookmark tag")
	return command
}

func newBookmarkReadCommand(
	dependencies commandDependencies,
	use string,
	short string,
	read bool,
) *cobra.Command {
	return &cobra.Command{
		Use: use + " [BOOKMARK_ID]", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Bookmark ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			current, err := client.GetBookmark(command.Context(), id)
			if err != nil {
				return fmt.Errorf("load bookmark before %s: %w", use, err)
			}
			bookmark, err := client.SetBookmarkRead(command.Context(), id, apiclient.BookmarkReadRequest{
				Read: read, ExpectedRevision: current.Revision,
			})
			if err != nil {
				return fmt.Errorf("%s bookmark: %w", use, err)
			}
			return dependencies.writeValue(command, bookmark)
		},
	}
}

func newBookmarkDeleteCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "delete [BOOKMARK_ID]", Short: "Delete a bookmark", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			id, err := parsePositiveCLIInteger("Bookmark ID", arguments[0])
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			if err := client.DeleteBookmark(command.Context(), id); err != nil {
				return fmt.Errorf("delete bookmark: %w", err)
			}
			return dependencies.writeSuccess(command, "Bookmark deleted.")
		},
	}
}

func newBookmarkTagsCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "tags", Short: "List bookmark tags", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			tags, err := client.ListBookmarkTags(command.Context())
			if err != nil {
				return fmt.Errorf("list bookmark tags: %w", err)
			}
			return dependencies.writeValue(command, tags)
		},
	}
}
