package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWallpapersCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "wallpapers", Short: "Inspect and change desktop wallpaper settings", Args: cobra.NoArgs}
	command.AddCommand(newWallpaperSettingsCommand(dependencies), newWallpaperSelectCommand(dependencies), newWallpaperDeleteCommand(dependencies))
	return command
}

func newWallpaperSettingsCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{Use: "settings", Short: "Show wallpaper settings and selections", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.WallpaperSettings(command.Context())
		if err != nil {
			return fmt.Errorf("load wallpaper settings: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
}

func newWallpaperSelectCommand(dependencies commandDependencies) *cobra.Command {
	var selection string
	command := &cobra.Command{Use: "select", Short: "Select a built-in or user wallpaper", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		prompt := newPrompter(command)
		if err := prompt.required("Selection", &selection); err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.SelectWallpaper(command.Context(), selection)
		if err != nil {
			return fmt.Errorf("select wallpaper: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&selection, "selection", "", "builtin:<name> or custom:<id> selection")
	return command
}

func newWallpaperDeleteCommand(dependencies commandDependencies) *cobra.Command {
	var id string
	command := &cobra.Command{Use: "delete [WALLPAPER_ID]", Short: "Permanently delete a user wallpaper", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if id == "" && len(args) == 1 {
			id = args[0]
		}
		prompt := newPrompter(command)
		if err := prompt.required("Wallpaper ID", &id); err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.DeleteUserWallpaper(command.Context(), id)
		if err != nil {
			return fmt.Errorf("delete user wallpaper: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&id, "id", "", "32-character user wallpaper ID")
	return command
}
