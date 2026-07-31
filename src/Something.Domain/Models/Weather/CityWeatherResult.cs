namespace Something.Domain.Models.Weather;

public sealed record CityWeatherResult(
    bool Found,
    WeatherLocation? Location,
    WeatherForecast? Forecast);
