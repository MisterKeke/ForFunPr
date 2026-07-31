namespace Something.Domain.Models.Weather;

public sealed record StoredWeatherResult(
    bool Found,
    WeatherForecast? Forecast,
    DateTimeOffset? UpdatedAt);
