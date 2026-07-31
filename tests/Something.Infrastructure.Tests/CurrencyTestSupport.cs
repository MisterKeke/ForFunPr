using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Abstractions.Providers;
using Something.Application.Features.Currencies;
using Something.Domain.Models.Currencies;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

internal sealed class CurrencyRepositoryTestHarness : IAsyncDisposable
{
    private readonly TemporaryApplicationPaths _paths;
    private readonly DatabaseInitializer _initializer;
    private readonly DatabaseWriteCoordinator _writeCoordinator;

    private CurrencyRepositoryTestHarness(
        TemporaryApplicationPaths paths,
        DatabaseInitializer initializer,
        DatabaseWriteCoordinator writeCoordinator,
        CurrencyService service)
    {
        _paths = paths;
        _initializer = initializer;
        _writeCoordinator = writeCoordinator;
        Service = service;
    }

    public CurrencyService Service { get; }

    public static async Task<CurrencyRepositoryTestHarness> CreateAsync(
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
        var repository = new CurrencyFavoriteRepository(connectionFactory, writeCoordinator);
        var service = new CurrencyService(
            new StubCurrencyProvider(), repository, new WeakReferenceMessenger());
        return new CurrencyRepositoryTestHarness(paths, initializer, writeCoordinator, service);
    }

    public async ValueTask DisposeAsync()
    {
        _initializer.Dispose();
        _writeCoordinator.Dispose();
        await _paths.DisposeAsync();
    }

    private sealed class StubCurrencyProvider : ICurrencyProvider
    {
        public Task<CurrencyRate> GetRateAsync(
            string baseCode,
            string targetCode,
            CancellationToken cancellationToken = default)
        {
            cancellationToken.ThrowIfCancellationRequested();
            return Task.FromResult(new CurrencyRate(
                baseCode, targetCode, 1.25m, new DateOnly(2026, 7, 31), true));
        }

        public Task<AllCurrencyRates> GetAllRatesAsync(
            string baseCode,
            CancellationToken cancellationToken = default)
        {
            cancellationToken.ThrowIfCancellationRequested();
            return Task.FromResult(new AllCurrencyRates(baseCode, null, []));
        }
    }
}

internal sealed class StubHttpClientFactory(HttpClient client) : IHttpClientFactory
{
    public HttpClient CreateClient(string name) => client;
}

internal sealed class DelegateHttpHandler(
    Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> handler) : HttpMessageHandler
{
    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        return handler(request, cancellationToken);
    }
}
