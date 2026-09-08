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
		Short: "List and manage favourite categories",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newFavoriteCategoryListCommand(dependencies),
		newFavoriteCategoryCreateCommand(dependencies),
		newFavoriteCategoryRenameCommand(dependencies),
		newFavoriteCategoryDeleteCommand(dependencies),
		newFavoriteCategoryReorderCommand(dependencies),
	)
	return command
}

func newFavoriteCategoryRenameCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var idText string
	var name string
	var color string
	var displayOrder int
	command := &cobra.Command{
		Use:     "update",
		Aliases: []string{"rename"},
		Short:   "Update a favourite category",
		Args:    cobra.NoArgs,
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
			request := apiclient.FavoriteCategoryUpdateRequest{Name: name}
			if command.Flags().Changed("color") {
				request.Color = &color
			}
			if command.Flags().Changed("display-order") {
				request.DisplayOrder = &displayOrder
			}
			result, err := client.UpdateFavoriteCategory(
				command.Context(),
				id,
				request,
			)
			if err != nil {
				return fmt.Errorf("update favourite category: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "category ID")
	command.Flags().StringVar(&name, "name", "", "new category name")
	command.Flags().StringVar(&color, "color", "", "hex category color; empty clears it")
	command.Flags().IntVar(&displayOrder, "display-order", 0, "non-negative display order")
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
	var color string
	var displayOrder int

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

			request := apiclient.FavoriteCategoryCreateRequest{Name: name, Source: source, Color: color}
			if command.Flags().Changed("display-order") {
				request.DisplayOrder = &displayOrder
			}
			result, err := client.CreateFavoriteCategory(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create favourite category: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&name, "name", "", "category name")
	command.Flags().StringVar(&color, "color", "", "optional hex category color")
	command.Flags().IntVar(&displayOrder, "display-order", 0, "optional non-negative display order")
	command.Flags().StringVar(
		&source,
		"source",
		"",
		"category source: telegram or youtube",
	)
	return command
}

func newFavoriteCategoryDeleteCommand(dependencies commandDependencies) *cobra.Command {
	var idText, mode, targetText string
	command := &cobra.Command{
		Use: "delete", Short: "Delete a favourite category", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := newPrompter(command).required("Category ID", &idText); err != nil {
				return err
			}
			id, err := parseInteger("Category ID", idText)
			if err != nil {
				return err
			}
			request := apiclient.FavoriteCategoryDeleteRequest{Mode: mode}
			if targetText != "" {
				target, err := parseInteger("Target category ID", targetText)
				if err != nil {
					return err
				}
				request.TargetCategoryID = &target
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.DeleteFavoriteCategory(command.Context(), id, request)
			if err != nil {
				return fmt.Errorf("delete favourite category: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "category ID")
	command.Flags().StringVar(&mode, "mode", "unassign", "unassign or move")
	command.Flags().StringVar(&targetText, "target-id", "", "same-source target category for move mode")
	return command
}

func newFavoriteCategoryReorderCommand(dependencies commandDependencies) *cobra.Command {
	var source string
	var ids []int
	command := &cobra.Command{
		Use: "reorder", Short: "Set favourite category display order", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if len(ids) == 0 {
				return fmt.Errorf("at least one --id is required")
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.ReorderFavoriteCategories(command.Context(), apiclient.FavoriteCategoryReorderRequest{Source: source, CategoryIDs: ids})
			if err != nil {
				return fmt.Errorf("reorder favourite categories: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&source, "source", "telegram", "telegram or youtube")
	command.Flags().IntSliceVar(&ids, "id", nil, "category IDs in desired order")
	return command
}
