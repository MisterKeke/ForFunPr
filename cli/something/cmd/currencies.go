package cmd

import (
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newCurrenciesCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "currencies",
		Short: "Read rates and manage currency favourites",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newCurrenciesListCommand(dependencies),
		newCurrencyRateCommand(dependencies),
		newCurrencyFavoritesCommand(dependencies),
	)
	return command
}

func newCurrenciesListCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var base string
	var symbols string

	command := &cobra.Command{
		Use:   "list",
		Short: "List currency rates",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Base", &base); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.Currencies(
				command.Context(),
				base,
				symbols,
			)
			if err != nil {
				return fmt.Errorf("list currency rates: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&base, "base", "", "base currency code")
	command.Flags().StringVar(
		&symbols,
		"symbols",
		"",
		"comma-separated target currency codes",
	)
	return command
}

func newCurrencyRateCommand(
	dependencies commandDependencies,
) *cobra.Command {
	var base string
	var target string

	command := &cobra.Command{
		Use:   "rate",
		Short: "Get one currency rate",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Base", &base); err != nil {
				return err
			}
			if err := prompt.required("Target", &target); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.CurrencyRate(
				command.Context(),
				base,
				target,
			)
			if err != nil {
				return fmt.Errorf("load currency rate: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&base, "base", "", "base currency code")
	command.Flags().StringVar(
		&target,
		"target",
		"",
		"target currency code",
	)
	return command
}

func newCurrencyFavoritesCommand(
	dependencies commandDependencies,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "favorites",
		Short: "Manage currency favourites",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newValueCommand(
			dependencies,
			"list",
			"List currency favourites",
			"list currency favourites",
			(*apiclient.Client).CurrencyFavorites,
		),
		newValueCommand(
			dependencies,
			"rates",
			"List favourite currency rates",
			"list favourite currency rates",
			(*apiclient.Client).CurrencyFavoriteRates,
		),
		newCurrencyFavoriteMutationCommand(
			dependencies,
			"add",
			"Add a currency favourite",
		),
		newCurrencyFavoriteMutationCommand(
			dependencies,
			"remove",
			"Remove a currency favourite",
		),
	)
	return command
}

func newCurrencyFavoriteMutationCommand(
	dependencies commandDependencies,
	action string,
	short string,
) *cobra.Command {
	var base string
	var target string

	command := &cobra.Command{
		Use:   action,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("Base", &base); err != nil {
				return err
			}
			if err := prompt.required("Target", &target); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			if action == "add" {
				result, err := client.AddCurrencyFavorite(
					command.Context(),
					base,
					target,
				)
				if err != nil {
					return fmt.Errorf("add currency favourite: %w", err)
				}
				return dependencies.writeValue(command, result)
			}

			if err := client.RemoveCurrencyFavorite(
				command.Context(),
				base,
				target,
			); err != nil {
				return fmt.Errorf("remove currency favourite: %w", err)
			}

			return dependencies.writeSuccess(
				command,
				"currency favourite removed",
			)
		},
	}
	command.Flags().StringVar(&base, "base", "", "base currency code")
	command.Flags().StringVar(
		&target,
		"target",
		"",
		"target currency code",
	)
	return command
}
