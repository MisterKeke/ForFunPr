using Something.Domain.Models.Weather;

namespace Something.Application.Abstractions.Persistence;

public interface IWeatherCacheRepository
{
    Task<StoredWeatherLocation> GetAsync(CancellationToken cancellationToken = default);
    Task<DateTimeOffset> SaveLocationAndForecastAsync(
        double latitude,
        double longitude,
        WeatherForecast forecast,
        CancellationToken cancellationToken = default);
    Task<DateTimeOffset> SaveForecastAsync(
        WeatherForecast forecast,
        CancellationToken cancellationToken = default);
}
