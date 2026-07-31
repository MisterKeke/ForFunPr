using Something.Domain.Models.Weather;

namespace Something.Application.Features.Weather;

public interface IWeatherService
{
    Task<WeatherForecast> GetForCoordinatesAsync(
        double latitude,
        double longitude,
        CancellationToken cancellationToken = default);
    Task<CityWeatherResult> GetForCityAsync(string city, CancellationToken cancellationToken = default);
    Task<StoredWeatherResult> GetStoredAsync(CancellationToken cancellationToken = default);
    Task<StoredWeatherResult> RefreshStoredAsync(CancellationToken cancellationToken = default);
}
