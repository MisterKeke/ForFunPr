package cmd

import (
	"fmt"
	"strings"

	"currency-wails/cli/internal/apiclient"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type taskListOptions struct {
	date     string
	settings *viper.Viper
}

func newTaskListCommand(settings *viper.Viper) *cobra.Command {
	options := taskListOptions{settings: settings}

	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE:  options.run,
	}
	command.Flags().StringVar(
		&options.date,
		"date",
		"",
		"only list tasks due on YYYY-MM-DD",
	)

	return command
}

func (options taskListOptions) run(
	command *cobra.Command,
	_ []string,
) error {
	client, err := apiclient.New(options.settings.GetString(apiURLConfigKey))
	if err != nil {
		return err
	}

	// Do not validate the date here. The API owns that rule and returns its
	// standard invalid_date error if the value is not a real YYYY-MM-DD date.
	tasks, err := client.ListTasks(
		command.Context(),
		strings.TrimSpace(options.date),
	)
	if err != nil {
		return fmt.Errorf(
			"list tasks (is the desktop app running?): %w",
			err,
		)
	}

	printTasks(command.OutOrStdout(), tasks)
	return nil
}
