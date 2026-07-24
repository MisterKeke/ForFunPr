package schemas

// WeatherCityInput selects a city for a live forecast.
type WeatherCityInput struct {
	City string `json:"city"`
}

// WeatherCityInputSchema is the explicit MCP schema for city weather lookup.
var WeatherCityInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"city": map[string]any{
			"type":        "string",
			"description": "Required city name between 1 and 100 characters.",
			"minLength":   1,
			"maxLength":   100,
		},
	},
	"required":             []string{"city"},
	"additionalProperties": false,
}

// WeatherLocationInput saves a location by coordinates.
type WeatherLocationInput struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// WeatherLocationInputSchema is the explicit MCP schema for saving weather
// coordinates.
var WeatherLocationInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"latitude": map[string]any{
			"type":        "number",
			"description": "Latitude from -90 through 90.",
			"minimum":     -90,
			"maximum":     90,
		},
		"longitude": map[string]any{
			"type":        "number",
			"description": "Longitude from -180 through 180.",
			"minimum":     -180,
			"maximum":     180,
		},
	},
	"required":             []string{"latitude", "longitude"},
	"additionalProperties": false,
}

// WeatherCurrent contains current Open-Meteo conditions.
type WeatherCurrent struct {
	Temperature2M       float64 `json:"temperature_2m" jsonschema:"Current temperature in Celsius."`
	ApparentTemperature float64 `json:"apparent_temperature" jsonschema:"Current feels-like temperature in Celsius."`
	WeatherCode         int     `json:"weather_code" jsonschema:"Open-Meteo weather code."`
	WindSpeed10M        float64 `json:"wind_speed_10m" jsonschema:"Wind speed at ten metres."`
}

// WeatherDaily contains aligned daily forecast arrays.
type WeatherDaily struct {
	Time                        []string  `json:"time" jsonschema:"Forecast dates."`
	WeatherCode                 []int     `json:"weather_code" jsonschema:"Daily weather codes."`
	Temperature2MMax            []float64 `json:"temperature_2m_max" jsonschema:"Daily maximum temperatures in Celsius."`
	Temperature2MMin            []float64 `json:"temperature_2m_min" jsonschema:"Daily minimum temperatures in Celsius."`
	PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max" jsonschema:"Daily maximum precipitation probabilities."`
}

// WeatherForecastOutput is the JSON emitted by the saved-location command.
type WeatherForecastOutput struct {
	Current  WeatherCurrent `json:"current" jsonschema:"Current conditions."`
	Daily    WeatherDaily   `json:"daily" jsonschema:"Multi-day forecast."`
	Timezone string         `json:"timezone" jsonschema:"Forecast timezone."`
}

// CityWeatherOutput is the projected JSON emitted by `weather get`.
type CityWeatherOutput struct {
	City                               string  `json:"city" jsonschema:"Matched city name."`
	Region                             string  `json:"region,omitempty" jsonschema:"Matched administrative region."`
	Country                            string  `json:"country,omitempty" jsonschema:"Matched country."`
	Date                               string  `json:"date" jsonschema:"Forecast date."`
	Timezone                           string  `json:"timezone" jsonschema:"Forecast timezone."`
	TemperatureC                       float64 `json:"temperature_c" jsonschema:"Current temperature in Celsius."`
	FeelsLikeC                         float64 `json:"feels_like_c" jsonschema:"Feels-like temperature in Celsius."`
	WeatherCode                        int     `json:"weather_code" jsonschema:"Open-Meteo weather code."`
	WindSpeedKMH                       float64 `json:"wind_speed_kmh" jsonschema:"Wind speed in kilometres per hour."`
	MinimumC                           float64 `json:"minimum_c" jsonschema:"Daily minimum temperature in Celsius."`
	MaximumC                           float64 `json:"maximum_c" jsonschema:"Daily maximum temperature in Celsius."`
	PrecipitationProbabilityPercentage float64 `json:"precipitation_probability_percentage" jsonschema:"Maximum precipitation probability percentage."`
}

// StoredWeatherOutput is the JSON emitted by stored and refresh commands.
type StoredWeatherOutput struct {
	Found     bool                   `json:"found" jsonschema:"Whether a saved location exists."`
	Weather   *WeatherForecastOutput `json:"weather,omitempty" jsonschema:"Cached or refreshed forecast when found."`
	UpdatedAt string                 `json:"updated_at,omitempty" jsonschema:"Timestamp of the cached forecast."`
}
