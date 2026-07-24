package writetools

import (
	"context"
	"strconv"

	"currency-wails/mcp-server/schemas"
	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterWeather adds saved-location mutation and refresh tools.
func RegisterWeather(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "save_weather_location",
		Title:       "Save weather location",
		Description: "Overwrite the saved coordinates and retrieve a live forecast for them.",
		InputSchema: schemas.WeatherLocationInputSchema,
		Annotations: tools.WriteAnnotations(true, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.WeatherLocationInput,
	) (*mcp.CallToolResult, schemas.WeatherForecastOutput, error) {
		if err := schemas.Coordinates(input.Latitude, input.Longitude); err != nil {
			return nil, schemas.WeatherForecastOutput{}, err
		}
		return tools.Execute[schemas.WeatherForecastOutput](
			ctx,
			runner,
			[]string{
				"weather", "save",
				"--latitude", strconv.FormatFloat(
					input.Latitude,
					'g',
					-1,
					64,
				),
				"--longitude", strconv.FormatFloat(
					input.Longitude,
					'g',
					-1,
					64,
				),
			},
			"Saved the location and loaded its live forecast.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "refresh_stored_weather",
		Title:       "Refresh stored weather",
		Description: "Replace the cached saved-location forecast with current live provider data.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.WriteAnnotations(true, false, true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.StoredWeatherOutput, error) {
		return tools.Execute[schemas.StoredWeatherOutput](
			ctx,
			runner,
			[]string{"weather", "refresh"},
			"Refreshed the forecast for the saved location.",
		)
	})
}
