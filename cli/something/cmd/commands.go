package cmd

import "github.com/spf13/cobra"

// addCommands is the single top-level command registry.
func addCommands(
	root *cobra.Command,
	dependencies commandDependencies,
) {
	root.AddCommand(
		newHealthCommand(dependencies),
		newNewsCommand(dependencies),
		newPostsCommand(dependencies),
		newFavoritesCommand(dependencies),
		newFavoriteCategoriesCommand(dependencies),
		newTasksCommand(dependencies),
		newWeatherCommand(dependencies),
		newCurrenciesCommand(dependencies),
	)
}
