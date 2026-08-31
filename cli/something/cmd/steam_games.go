package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSteamGamesCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{Use: "steam-games", Short: "Manage tracked Steam games and prices", Args: cobra.NoArgs}
	command.AddCommand(
		newSteamGamesListCommand(dependencies), newSteamGameAddCommand(dependencies),
		newSteamGameDeleteCommand(dependencies), newSteamGameSettingsCommand(dependencies),
		newSteamGameCountryCommand(dependencies), newSteamCountriesCommand(dependencies),
		newSteamGamesRefreshCommand(dependencies),
	)
	return command
}

func newSteamGamesListCommand(dependencies commandDependencies) *cobra.Command {
	return steamValueCommand(dependencies, "list", "List tracked Steam games", "list Steam games", func(command *cobra.Command) (any, error) {
		client, err := dependencies.client()
		if err != nil {
			return nil, err
		}
		return client.SteamGames(command.Context())
	})
}

func newSteamGameSettingsCommand(dependencies commandDependencies) *cobra.Command {
	return steamValueCommand(dependencies, "settings", "Show Steam price country settings", "load Steam settings", func(command *cobra.Command) (any, error) {
		client, err := dependencies.client()
		if err != nil {
			return nil, err
		}
		return client.SteamGameSettings(command.Context())
	})
}

func newSteamCountriesCommand(dependencies commandDependencies) *cobra.Command {
	return steamValueCommand(dependencies, "countries", "List supported Steam store countries", "list Steam countries", func(command *cobra.Command) (any, error) {
		client, err := dependencies.client()
		if err != nil {
			return nil, err
		}
		return client.SteamCountries(command.Context())
	})
}

func newSteamGamesRefreshCommand(dependencies commandDependencies) *cobra.Command {
	return steamValueCommand(dependencies, "refresh", "Refresh every tracked Steam price", "refresh Steam games", func(command *cobra.Command) (any, error) {
		client, err := dependencies.client()
		if err != nil {
			return nil, err
		}
		return client.RefreshSteamGames(command.Context())
	})
}

func steamValueCommand(dependencies commandDependencies, use string, short string, operation string, call func(*cobra.Command) (any, error)) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		result, err := call(command)
		if err != nil {
			return fmt.Errorf("%s: %w", operation, err)
		}
		return dependencies.writeValue(command, result)
	}}
}

func newSteamGameAddCommand(dependencies commandDependencies) *cobra.Command {
	var storeURL string
	command := &cobra.Command{Use: "add", Short: "Track a Steam Store game URL", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		prompt := newPrompter(command)
		if err := prompt.required("Steam Store URL", &storeURL); err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.AddSteamGame(command.Context(), storeURL)
		if err != nil {
			return fmt.Errorf("add Steam game: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&storeURL, "url", "", "Steam Store /app/ URL")
	return command
}

func newSteamGameDeleteCommand(dependencies commandDependencies) *cobra.Command {
	var idText string
	command := &cobra.Command{Use: "delete [GAME_ID]", Short: "Delete a tracked Steam game", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if idText == "" && len(args) == 1 {
			idText = args[0]
		}
		prompt := newPrompter(command)
		if err := prompt.required("Game ID", &idText); err != nil {
			return err
		}
		id, err := parseInteger("Game ID", idText)
		if err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		if err := client.DeleteSteamGame(command.Context(), id); err != nil {
			return fmt.Errorf("delete Steam game: %w", err)
		}
		return dependencies.writeSuccess(command, "Steam game deleted")
	}}
	command.Flags().StringVar(&idText, "id", "", "tracked Steam game ID")
	return command
}

func newSteamGameCountryCommand(dependencies commandDependencies) *cobra.Command {
	var country string
	command := &cobra.Command{Use: "set-country", Short: "Set the Steam Store price country", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		prompt := newPrompter(command)
		if err := prompt.required("Country code", &country); err != nil {
			return err
		}
		client, err := dependencies.client()
		if err != nil {
			return err
		}
		result, err := client.SetSteamGameCountry(command.Context(), country)
		if err != nil {
			return fmt.Errorf("set Steam country: %w", err)
		}
		return dependencies.writeValue(command, result)
	}}
	command.Flags().StringVar(&country, "country", "", "supported two-letter country code")
	return command
}
