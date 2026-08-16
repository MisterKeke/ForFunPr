package cmd

import (
	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newHealthCommand(
	dependencies commandDependencies,
) *cobra.Command {
	return newValueCommand(
		dependencies,
		"health",
		"Show API health",
		"load health",
		(*apiclient.Client).Health,
	)
}
