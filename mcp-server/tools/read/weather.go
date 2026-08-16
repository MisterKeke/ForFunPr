package readtools

import (
	"context"
	"unicode/utf8"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterWeather adds live and stored weather read tools.
func RegisterWeather(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "get_weather",
		Title:       "Get city weather",
		Description: "Look up a city and retrieve its current live Open-Meteo forecast.",
		InputSchema: schemas.WeatherCityInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.WeatherCityInput,
	) (*mcp.CallToolResult, schemas.CityWeatherOutput, error) {
		city, err := schemas.RequiredString("city", input.City)
		if err != nil {
			return nil, schemas.CityWeatherOutput{}, err
		}
		if !utf8.ValidString(city) || utf8.RuneCountInString(city) > 100 {
			return nil, schemas.CityWeatherOutput{},
				&weatherInputError{"city must contain between 1 and 100 characters"}
		}

		return tools.Execute[schemas.CityWeatherOutput](
			ctx,
			runner,
			[]string{"weather", "get", "--city", city},
			"Loaded live weather for the requested city.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "get_stored_weather",
		Title:       "Get stored weather",
		Description: "Read the locally cached forecast for the saved location without refreshing it.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.StoredWeatherOutput, error) {
		return tools.Execute[schemas.StoredWeatherOutput](
			ctx,
			runner,
			[]string{"weather", "stored"},
			"Loaded the cached forecast for the saved location.",
		)
	})
}

type weatherInputError struct {
	message string
}

func (err *weatherInputError) Error() string {
	return err.message
}
