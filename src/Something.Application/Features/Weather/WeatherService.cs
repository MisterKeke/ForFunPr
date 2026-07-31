using Something.Application.Abstractions.Persistence;
using Something.Application.Abstractions.Providers;
using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;
using Something.Domain.Validation;

namespace Something.Application.Features.Weather;

public sealed class WeatherService(
    IWeatherProvider provider,
    IWeatherCacheRepository cacheRepository) : IWeatherService
{
    public async Task<WeatherForecast> GetForCoordinatesAsync(
        double latitude,
        double longitude,
        CancellationToken cancellationToken = default)
    {
        WeatherRules.ValidateCoordinates(latitude, longitude);
        var forecast = await provider.GetForecastAsync(latitude, longitude, cancellationToken);
        WeatherRules.ValidateForecast(forecast);
        await cacheRepository.SaveLocationAndForecastAsync(
            latitude, longitude, forecast, cancellationToken);
        return forecast;
    }

    public async Task<CityWeatherResult> GetForCityAsync(
        string city,
        CancellationToken cancellationToken = default)
    {
        city = WeatherRules.NormalizeCity(city);
        var location = await provider.FindCityAsync(city, cancellationToken);
        if (location is null)
        {
            return new CityWeatherResult(false, null, null);
        }

        WeatherRules.ValidateCoordinates(location.Latitude, location.Longitude);
        var forecast = await provider.GetForecastAsync(
            location.Latitude, location.Longitude, cancellationToken);
        WeatherRules.ValidateForecast(forecast);
        return new CityWeatherResult(true, location, forecast);
    }

    public async Task<StoredWeatherResult> GetStoredAsync(CancellationToken cancellationToken = default)
    {
        var stored = await cacheRepository.GetAsync(cancellationToken);
        if (!stored.Found)
        {
            return new StoredWeatherResult(false, null, null);
        }

        WeatherRules.ValidateCoordinates(stored.Latitude, stored.Longitude);
        return new StoredWeatherResult(true, stored.Forecast, stored.UpdatedAt);
    }

    public async Task<StoredWeatherResult> RefreshStoredAsync(CancellationToken cancellationToken = default)
    {
        var stored = await cacheRepository.GetAsync(cancellationToken);
        if (!stored.Found)
        {
            return new StoredWeatherResult(false, null, null);
        }

        WeatherRules.ValidateCoordinates(stored.Latitude, stored.Longitude);
        var forecast = await provider.GetForecastAsync(
            stored.Latitude, stored.Longitude, cancellationToken);
        WeatherRules.ValidateForecast(forecast);
        var updatedAt = await cacheRepository.SaveForecastAsync(forecast, cancellationToken);
        return new StoredWeatherResult(true, forecast, updatedAt);
    }
}
