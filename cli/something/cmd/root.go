package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	apiURLConfigKey = "api-url"
	defaultAPIURL   = "http://127.0.0.1:8080"
)

func Execute(ctx context.Context) error {
	root, err := newRootCommand()
	if err != nil {
		return err
	}

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
