package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDesktopAppsCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "desktop-apps", Short: "Inspect, rename, and launch saved desktop applications", Args: cobra.NoArgs}
	command.AddCommand(
		newDesktopAppsListCommand(dependencies),
		newDesktopAppRenameCommand(dependencies),
		newDesktopAppLaunchCommand(dependencies),
	)
	return command
}

func newDesktopAppLaunchCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	var confirmed bool
	command := &cobra.Command{
		Use:   "launch [APP_ID]",
		Short: "Launch a saved desktop application",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if idText == "" && len(args) == 1 {
				idText = args[0]
			}
			prompt := newPrompter(command)
			if err := prompt.required("Application ID", &idText); err != nil {
				return err
			}
			if err := requireExecutionConfirmation(confirmed); err != nil {
				return err
			}
			id, err := parseInteger("Application ID", idText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.LaunchDesktopApp(command.Context(), id, confirmed)
			if err != nil {
				return fmt.Errorf("launch desktop application: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "saved desktop application ID")
	command.Flags().BoolVar(&confirmed, "confirm", false, "confirm that the saved application may be launched")
	return command
}

func newDesktopAppsListCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List saved desktop applications without filesystem paths", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.DesktopApps(command.Context())
			if err != nil {
				return fmt.Errorf("list desktop applications: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
}

func newDesktopAppRenameCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	var name string
	command := &cobra.Command{
		Use: "rename [APP_ID]", Short: "Rename a saved desktop application", Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if idText == "" && len(args) == 1 {
				idText = args[0]
			}
			prompt := newPrompter(command)
			if err := prompt.required("Application ID", &idText); err != nil {
				return err
			}
			if err := prompt.required("Display name", &name); err != nil {
				return err
			}
			id, err := parseInteger("Application ID", idText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.RenameDesktopApp(command.Context(), id, name)
			if err != nil {
				return fmt.Errorf("rename desktop application: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "desktop application ID")
	command.Flags().StringVar(&name, "name", "", "replacement display name")
	return command
}
