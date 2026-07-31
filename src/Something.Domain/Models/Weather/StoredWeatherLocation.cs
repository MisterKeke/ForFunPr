namespace Something.Domain.Models.Weather;

public sealed record StoredWeatherLocation(
    bool Found,
    double Latitude,
    double Longitude,
    WeatherForecast? Forecast,
    DateTimeOffset? UpdatedAt);
