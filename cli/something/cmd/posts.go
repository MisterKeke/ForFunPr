package cmd

import (
	"fmt"
	"strings"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newPostsCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "posts",
		Short: "List Telegram or YouTube posts",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newPostSourceCommand(dependencies, "telegram"),
		newPostSourceCommand(dependencies, "youtube"),
		newFavoritePostsCommand(dependencies),
	)
	return command
}

func newPostSourceCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	var channel string
	var before string

	command := &cobra.Command{
		Use:   source,
		Short: fmt.Sprintf("List %s posts", source),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Channel", &channel); err != nil {
				return err
			}

			var cursor *int
			if source == "telegram" && strings.TrimSpace(before) != "" {
				parsed, err := parseInteger("Before", before)
				if err != nil {
					return err
				}
				cursor = &parsed
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			var posts []apiclient.Post
			switch source {
			case "telegram":
				posts, err = client.TelegramPosts(
					command.Context(),
					channel,
					cursor,
				)
			case "youtube":
				posts, err = client.YouTubePosts(
					command.Context(),
					channel,
					cursor,
				)
			}
			if err != nil {
				return fmt.Errorf("list %s posts: %w", source, err)
			}

			return writePostsOutput(
				command.OutOrStdout(),
				dependencies.outputFormat(),
				source,
				channel,
				posts,
			)
		},
	}
	command.Flags().StringVar(&channel, "channel", "", "channel name")
	if source == "telegram" {
		command.Flags().StringVar(&before, "before", "", "non-negative pagination cursor")
	}
	return command
}

func newFavoritePostsCommand(
	dependencies commandDependencies,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "favorites",
		Short: "List posts from saved favourite channels",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newFavoritePostSourceCommand(dependencies, "telegram"),
		newFavoritePostSourceCommand(dependencies, "youtube"),
	)
	return command
}

func newFavoritePostSourceCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	var before string

	command := &cobra.Command{
		Use:   source,
		Short: fmt.Sprintf("List posts from %s favourites", source),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			var cursor *int
			if source == "telegram" && strings.TrimSpace(before) != "" {
				parsed, err := parseInteger("Before", before)
				if err != nil {
					return err
				}
				cursor = &parsed
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			var posts []apiclient.Post
			switch source {
			case "telegram":
				posts, err = client.FavoriteTelegramPosts(
					command.Context(),
					cursor,
				)
			case "youtube":
				posts, err = client.FavoriteYouTubePosts(
					command.Context(),
					cursor,
				)
			}
			if err != nil {
				return fmt.Errorf(
					"list posts from %s favourites: %w",
					source,
					err,
				)
			}

			return writePostsOutput(
				command.OutOrStdout(),
				dependencies.outputFormat(),
				source,
				"",
				posts,
			)
		},
	}
	if source == "telegram" {
		command.Flags().StringVar(&before, "before", "", "non-negative pagination cursor")
	}
	return command
}
