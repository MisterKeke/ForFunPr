using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Abstractions.Providers;
using Something.Application.Features.Weather;
using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

internal sealed class WeatherRepositoryTestHarness : IAsyncDisposable
{
    private readonly TemporaryApplicationPaths _paths;
    private readonly DatabaseInitializer _initializer;
    private readonly DatabaseWriteCoordinator _writeCoordinator;

    private WeatherRepositoryTestHarness(
        TemporaryApplicationPaths paths,
        DatabaseInitializer initializer,
        DatabaseWriteCoordinator writeCoordinator,
        WeatherService service,
        MutableWeatherProvider provider)
    {
        _paths = paths;
        _initializer = initializer;
        _writeCoordinator = writeCoordinator;
        Service = service;
        Provider = provider;
    }

    public WeatherService Service { get; }
    public MutableWeatherProvider Provider { get; }

    public static async Task<WeatherRepositoryTestHarness> CreateAsync(
        CancellationToken cancellationToken)
    {
        var paths = TemporaryApplicationPaths.Create();
        var connectionFactory = new SqliteConnectionFactory(paths);
        var migrations = new IMigration[]
        {
            new Migration001CoreSchema(), new Migration002FavoriteSchema(),
            new Migration003ApplicationState(), new Migration004FavoriteUpdates(),
            new Migration005WeatherLocation(), new Migration006WeatherCache(),
            new Migration007FavoriteNews(), new Migration008TaskMetadata(),
            new Migration009Wallpapers(), new Migration010Notes(),
            new Migration011Bookmarks(),
        };
        var initializer = new DatabaseInitializer(
            paths,
            connectionFactory,
            migrations,
            NullLogger<DatabaseInitializer>.Instance);
        await initializer.InitializeAsync(cancellationToken);
        var writeCoordinator = new DatabaseWriteCoordinator();
        var repository = new WeatherCacheRepository(connectionFactory, writeCoordinator);
        var provider = new MutableWeatherProvider();
        var service = new WeatherService(provider, repository);
        return new WeatherRepositoryTestHarness(
            paths, initializer, writeCoordinator, service, provider);
    }

    public async ValueTask DisposeAsync()
    {
        _initializer.Dispose();
        _writeCoordinator.Dispose();
        await _paths.DisposeAsync();
    }
}

internal sealed class MutableWeatherProvider : IWeatherProvider
{
    public WeatherForecast Forecast { get; set; } = WeatherTestData.Forecast(20);
    public bool FailForecast { get; set; }
    public WeatherLocation? Location { get; set; } = new("Istanbul", "Istanbul", "Türkiye", 41.01, 28.97);

    public Task<WeatherForecast> GetForecastAsync(
        double latitude,
        double longitude,
        CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        if (FailForecast)
        {
            throw new ProviderUnavailableException("Weather unavailable.");
        }

        return Task.FromResult(Forecast);
    }

    public Task<WeatherLocation?> FindCityAsync(
        string city,
        CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        return Task.FromResult(Location);
    }
}

internal static class WeatherTestData
{
    public static WeatherForecast Forecast(double temperature)
    {
        return new WeatherForecast(
            new WeatherCurrent(temperature, temperature - 1, 1, 12),
            [new WeatherDay(new DateOnly(2026, 7, 31), 2, temperature + 3, temperature - 3, 25)],
            "Europe/Istanbul");
    }
}
