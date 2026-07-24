package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newTasksCommand(settings *viper.Viper) *cobra.Command {
	command := &cobra.Command{
		Use:   "tasks",
		Short: "Work with tasks",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(newTaskListCommand(settings))
	return command
}
