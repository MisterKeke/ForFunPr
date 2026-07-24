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
	if err := settings.BindPFlag(
		apiURLConfigKey,
		root.PersistentFlags().Lookup(apiURLConfigKey),
	); err != nil {
		return nil, fmt.Errorf("bind API URL configuration: %w", err)
	}

	root.AddCommand(newTasksCommand(settings))
	return root, nil
}
