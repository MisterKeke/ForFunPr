package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	apiURLConfigKey = "api-url"
	defaultAPIURL   = "http://127.0.0.1:8080"
)

var disableProgrammaticMousetrap sync.Once

func Execute(ctx context.Context) error {
	root, err := newRootCommand()
	if err != nil {
		return err
	}

	return root.ExecuteContext(ctx)
}

// ExecuteArgs runs a fresh Something command graph with the supplied
// arguments and streams. It does not read or modify os.Args.
func ExecuteArgs(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	// Cobra's Windows pre-execution hook assumes an Explorer-launched process
	// is a standalone CLI. Something.exe is Explorer-launched too, so allowing
	// that hook to run from the in-process MCP runner waits five seconds and
	// then terminates the entire desktop application with os.Exit(1).
	disableProgrammaticMousetrap.Do(func() {
		cobra.MousetrapHelpText = ""
	})

	root, err := newRootCommand()
	if err != nil {
		return err
	}

	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	// Programmatic callers must never wait for an interactive prompt.
	root.SetIn(strings.NewReader(""))

	return root.ExecuteContext(ctx)
}

func newRootCommand() (*cobra.Command, error) {
	settings := viper.New()
	settings.SetDefault(apiURLConfigKey, defaultAPIURL)
	settings.SetDefault(outputConfigKey, defaultOutputFormat)
	settings.SetEnvPrefix("SOMETHING")
	settings.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	settings.AutomaticEnv()

	root := &cobra.Command{
		Use:           "something",
		Short:         "Command-line access to the Something desktop API",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().String(
		apiURLConfigKey,
		defaultAPIURL,
		"base URL of the running desktop app's API",
	)
	root.PersistentFlags().StringP(
		outputConfigKey,
		"o",
		defaultOutputFormat,
		"output format: table or json",
	)

	for _, key := range []string{apiURLConfigKey, outputConfigKey} {
		if err := settings.BindPFlag(
			key,
			root.PersistentFlags().Lookup(key),
		); err != nil {
			return nil, fmt.Errorf("bind %s configuration: %w", key, err)
		}
	}

	addCommands(root, commandDependencies{settings: settings})
	return root, nil
}
