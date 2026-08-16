package cmd

import (
	"context"
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

type newsCall func(
	*apiclient.Client,
	context.Context,
) (apiclient.NewsResponse, error)

func newNewsCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "news",
		Short: "Read and refresh favourite-channel news",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newNewsOutputCommand(
			dependencies,
			"list",
			"List the latest stored news scan without refreshing providers",
			"list news",
			(*apiclient.Client).News,
		),
		newNewsOutputCommand(
			dependencies,
			"initial",
			"Run the initial favourite update scan",
			"scan initial news",
			(*apiclient.Client).InitialNews,
		),
		newNewsOutputCommand(
			dependencies,
			"refresh",
			"Refresh favourite-channel news",
			"refresh news",
			(*apiclient.Client).RefreshNews,
		),
		newNewsOutputCommand(
			dependencies,
			"since-last-open",
			"Scan news published since the previous app open",
			"scan news since last open",
			(*apiclient.Client).NewsSinceLastOpen,
		),
		newValueCommand(
			dependencies,
			"state",
			"Show news refresh state",
			"load news state",
			(*apiclient.Client).NewsState,
		),
		newValueCommand(
			dependencies,
			"windows",
			"Show news update windows",
			"load news windows",
			(*apiclient.Client).NewsWindows,
		),
	)

	return command
}

func newNewsOutputCommand(
	dependencies commandDependencies,
	use string,
	short string,
	operation string,
	call newsCall,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := call(client, command.Context())
			if err != nil {
				return fmt.Errorf("%s: %w", operation, err)
			}

			return writeNewsOutput(
				command.OutOrStdout(),
				dependencies.outputFormat(),
				result,
			)
		},
	}
}
