package cmd

import (
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newSetupsCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "setups", Short: "Manage saved application setups", Args: cobra.NoArgs}
	command.AddCommand(
		newSetupsListCommand(dependencies),
		newSetupCreateCommand(dependencies),
		newSetupUpdateCommand(dependencies),
		newSetupDeleteCommand(dependencies),
	)
	return command
}

func newSetupsListCommand(dependencies commandDependencies) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List saved setups", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.Setups(command.Context())
			if err != nil {
				return fmt.Errorf("list setups: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
}

func newSetupCreateCommand(dependencies commandDependencies) *cobra.Command {
	var request apiclient.SetupWriteRequest
	command := &cobra.Command{
		Use: "create", Short: "Create a setup without uploading an icon", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Name", &request.Name); err != nil {
				return err
			}
			if len(request.AppIDs) == 0 {
				return fmt.Errorf("at least one --app-id is required")
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.CreateSetup(command.Context(), request)
			if err != nil {
				return fmt.Errorf("create setup: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	addSetupWriteFlags(command, &request, false)
	return command
}

func newSetupUpdateCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	var request apiclient.SetupWriteRequest
	command := &cobra.Command{
		Use: "update [SETUP_ID]", Short: "Replace a setup's name, description, and applications", Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if idText == "" && len(args) == 1 {
				idText = args[0]
			}
			prompt := newPrompter(command)
			if err := prompt.required("Setup ID", &idText); err != nil {
				return err
			}
			if err := prompt.required("Name", &request.Name); err != nil {
				return err
			}
			if len(request.AppIDs) == 0 {
				return fmt.Errorf("at least one --app-id is required")
			}
			id, err := parseInteger("Setup ID", idText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			result, err := client.UpdateSetup(command.Context(), id, request)
			if err != nil {
				return fmt.Errorf("update setup: %w", err)
			}
			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&idText, "id", "", "setup ID")
	addSetupWriteFlags(command, &request, true)
	return command
}

func newSetupDeleteCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	command := &cobra.Command{
		Use: "delete [SETUP_ID]", Short: "Permanently delete a setup", Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if idText == "" && len(args) == 1 {
				idText = args[0]
			}
			prompt := newPrompter(command)
			if err := prompt.required("Setup ID", &idText); err != nil {
				return err
			}
			id, err := parseInteger("Setup ID", idText)
			if err != nil {
				return err
			}
			client, err := dependencies.client()
			if err != nil {
				return err
			}
			if err := client.DeleteSetup(command.Context(), id); err != nil {
				return fmt.Errorf("delete setup: %w", err)
			}
			return dependencies.writeSuccess(command, "Setup deleted.")
		},
	}
	command.Flags().StringVar(&idText, "id", "", "setup ID")
	return command
}

func addSetupWriteFlags(command *cobra.Command, request *apiclient.SetupWriteRequest, allowRemoveIcon bool) {
	command.Flags().StringVar(&request.Name, "name", "", "setup name")
	command.Flags().StringVar(&request.Description, "description", "", "setup description")
	command.Flags().IntSliceVar(&request.AppIDs, "app-id", nil, "desktop application ID; may be repeated")
	if allowRemoveIcon {
		command.Flags().BoolVar(&request.RemoveIcon, "remove-icon", false, "remove the setup's current icon")
	}
}
