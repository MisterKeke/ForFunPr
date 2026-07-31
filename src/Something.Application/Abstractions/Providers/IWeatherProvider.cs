using Something.Domain.Models.Weather;

namespace Something.Application.Abstractions.Providers;

public interface IWeatherProvider
{
    Task<WeatherForecast> GetForecastAsync(
        double latitude,
        double longitude,
        CancellationToken cancellationToken = default);

    Task<WeatherLocation?> FindCityAsync(
        string city,
        CancellationToken cancellationToken = default);
}
