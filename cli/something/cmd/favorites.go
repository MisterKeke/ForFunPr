package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFavoritesCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "favorites",
		Short: "Manage Telegram and YouTube favourites",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newFavoriteSourceCommand(dependencies, "telegram"),
		newFavoriteSourceCommand(dependencies, "youtube"),
	)
	return command
}

func newFavoriteSourceCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	command := &cobra.Command{
		Use:   source,
		Short: fmt.Sprintf("Manage %s favourites", source),
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newFavoriteListCommand(dependencies, source, false),
		newFavoriteListCommand(dependencies, source, true),
		newFavoriteAddCommand(dependencies, source),
		newFavoriteRemoveCommand(dependencies, source),
		newFavoriteAssignCommand(dependencies, source),
	)
	return command
}

func newFavoriteListCommand(
	dependencies commandDependencies,
	source string,
	categorized bool,
) *cobra.Command {
	use := "list"
	short := fmt.Sprintf("List %s favourites", source)
	if categorized {
		use = "list-categories"
		short = fmt.Sprintf("List categorized %s favourites", source)
	}

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.Favorites(
				command.Context(),
				source,
				categorized,
			)
			if err != nil {
				return fmt.Errorf("list %s favourites: %w", source, err)
			}

			return dependencies.writeValue(command, result)
		},
	}
}

func newFavoriteAddCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	var channel string

	command := &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("Add a %s favourite", source),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Channel", &channel); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.AddFavoriteChannel(
				command.Context(),
				source,
				channel,
			)
			if err != nil {
				return fmt.Errorf("add %s favourite: %w", source, err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&channel, "channel", "", "channel name")
	return command
}

func newFavoriteRemoveCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	var channel string

	command := &cobra.Command{
		Use:   "remove",
		Short: fmt.Sprintf("Remove a %s favourite", source),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Channel", &channel); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			if err := client.RemoveFavoriteChannel(
				command.Context(),
				source,
				channel,
			); err != nil {
				return fmt.Errorf("remove %s favourite: %w", source, err)
			}

			return dependencies.writeSuccess(
				command,
				fmt.Sprintf("%s favourite removed", source),
			)
		},
	}
	command.Flags().StringVar(&channel, "channel", "", "channel name")
	return command
}

func newFavoriteAssignCommand(
	dependencies commandDependencies,
	source string,
) *cobra.Command {
	var channel string
	var categoryID string

	command := &cobra.Command{
		Use:   "assign-category",
		Short: fmt.Sprintf("Assign a category to a %s favourite", source),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Channel", &channel); err != nil {
				return err
			}
			if err := prompt.required("Category ID", &categoryID); err != nil {
				return err
			}

			id, err := parseInteger("Category ID", categoryID)
			if err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			if err := client.AssignFavoriteCategory(
				command.Context(),
				source,
				channel,
				id,
			); err != nil {
				return fmt.Errorf(
					"assign %s favourite category: %w",
					source,
					err,
				)
			}

			return dependencies.writeSuccess(
				command,
				fmt.Sprintf("%s favourite category assigned", source),
			)
		},
	}
	command.Flags().StringVar(&channel, "channel", "", "channel name")
	command.Flags().StringVar(
		&categoryID,
		"category-id",
		"",
		"favourite category ID",
	)
	return command
}
