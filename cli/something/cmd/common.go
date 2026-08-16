package cmd

import (
	"context"
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type commandDependencies struct {
	settings *viper.Viper
}

type valueCall func(*apiclient.Client, context.Context) (any, error)

func newValueCommand(
	dependencies commandDependencies,
	use string,
	short string,
	operation string,
	call valueCall,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return dependencies.runValue(command, operation, call)
		},
	}
}

func (dependencies commandDependencies) client() (*apiclient.Client, error) {
	if err := validateOutputFormat(dependencies.outputFormat()); err != nil {
		return nil, err
	}
	return apiclient.New(
		dependencies.settings.GetString(apiURLConfigKey),
	)
}

func (dependencies commandDependencies) outputFormat() string {
	return dependencies.settings.GetString(outputConfigKey)
}

func (dependencies commandDependencies) runValue(
	command *cobra.Command,
	operation string,
	call valueCall,
) error {
	client, err := dependencies.client()
	if err != nil {
		return err
	}

	result, err := call(client, command.Context())
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	return dependencies.writeValue(command, result)
}

func (dependencies commandDependencies) writeValue(
	command *cobra.Command,
	value any,
) error {
	return writeValueOutput(
		command.OutOrStdout(),
		dependencies.outputFormat(),
		value,
	)
}

func (dependencies commandDependencies) writeSuccess(
	command *cobra.Command,
	message string,
) error {
	return writeSuccessOutput(
		command.OutOrStdout(),
		dependencies.outputFormat(),
		message,
	)
}
