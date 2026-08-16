package cmd

import (
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newFavoriteCategoriesCommand(
	dependencies commandDependencies,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "favorite-categories",
		Short: "List, create, and rename favourite categories",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newFavoriteCategoryListCommand(dependencies),
		newFavoriteCategoryCreateCommand(dependencies),
		newFavoriteCategoryRenameCommand(dependencies),
	)
	return command
}

func newFavoriteCategoryRenameCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var idText string
	var name string
	command := &cobra.Command{
		Use:   "rename",
		Short: "Rename a favourite category",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Category ID", &idText); err != nil {
				return err
			}
			if err := prompt.required("Name", &name); err != nil {
				return err
			}
			id, err := parseInteger("Category ID", idText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.RenameFavoriteCategory(
				command.Context(),
				id,
				apiclient.FavoriteCategoryRenameRequest{Name: name},
			)
			if err != nil {
				return fmt.Errorf("rename favourite category: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "category ID")
	command.Flags().StringVar(&name, "name", "", "new category name")
	return command
}

func newFavoriteCategoryListCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var source string

	command := &cobra.Command{
		Use:   "list",
		Short: "List favourite categories",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.FavoriteCategories(
				command.Context(),
				source,
			)
			if err != nil {
				return fmt.Errorf("list favourite categories: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(
		&source,
		"source",
		"",
		"category source: telegram or youtube (default telegram)",
	)
	return command
}

func newFavoriteCategoryCreateCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var name string
	var source string

	command := &cobra.Command{
		Use:   "create",
		Short: "Create a favourite category",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Name", &name); err != nil {
				return err
			}
			if err := prompt.required("Source", &source); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.CreateFavoriteCategory(
				command.Context(),
				apiclient.FavoriteCategoryCreateRequest{
					Name:   name,
					Source: source,
				},
			)
			if err != nil {
				return fmt.Errorf("create favourite category: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&name, "name", "", "category name")
	command.Flags().StringVar(
		&source,
		"source",
		"",
		"category source: telegram or youtube",
	)
	return command
}
