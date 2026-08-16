package cmd

import (
	"fmt"

	"something/cli/internal/apiclient"

	"github.com/spf13/cobra"
)

func newWeatherCommand(dependencies commandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "weather",
		Short: "Read and update weather data",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newWeatherGetCommand(dependencies),
		newWeatherSaveCommand(dependencies),
		newValueCommand(
			dependencies,
			"stored",
			"Show weather for the stored location",
			"load stored weather",
			(*apiclient.Client).StoredWeather,
		),
		newValueCommand(
			dependencies,
			"refresh",
			"Refresh weather for the stored location",
			"refresh stored weather",
			(*apiclient.Client).RefreshStoredWeather,
		),
	)
	return command
}

func newWeatherGetCommand(dependencies commandDependencies) *cobra.Command {
	var city string

	command := &cobra.Command{
		Use:   "get",
		Short: "Get weather for a city",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required("City", &city); err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.Weather(command.Context(), city)
			if err != nil {
				return fmt.Errorf("load weather: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(&city, "city", "", "city name")
	return command
}

func newWeatherSaveCommand(dependencies commandDependencies) *cobra.Command {
	var latitudeText string
	var longitudeText string

	command := &cobra.Command{
		Use:   "save",
		Short: "Save a weather location by coordinates",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			prompt := newPrompter(command)
			if err := prompt.required(
				"Latitude",
				&latitudeText,
			); err != nil {
				return err
			}
			if err := prompt.required(
				"Longitude",
				&longitudeText,
			); err != nil {
				return err
			}

			latitude, err := parseFloat("Latitude", latitudeText)
			if err != nil {
				return err
			}
			longitude, err := parseFloat("Longitude", longitudeText)
			if err != nil {
				return err
			}

			client, err := dependencies.client()
			if err != nil {
				return err
			}

			result, err := client.SaveWeatherLocation(
				command.Context(),
				apiclient.WeatherLocationRequest{
					Latitude:  latitude,
					Longitude: longitude,
				},
			)
			if err != nil {
				return fmt.Errorf("save weather location: %w", err)
			}

			return dependencies.writeValue(command, result)
		},
	}
	command.Flags().StringVar(
		&latitudeText,
		"latitude",
		"",
		"location latitude",
	)
	command.Flags().StringVar(
		&longitudeText,
		"longitude",
		"",
		"location longitude",
	)
	return command
}
