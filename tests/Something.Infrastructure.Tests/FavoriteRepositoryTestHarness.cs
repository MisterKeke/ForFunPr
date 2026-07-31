using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Features.Favorites;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

internal sealed class FavoriteRepositoryTestHarness : IAsyncDisposable
{
    private readonly TemporaryApplicationPaths _paths;
    private readonly DatabaseInitializer _initializer;
    private readonly DatabaseWriteCoordinator _writeCoordinator;

    private FavoriteRepositoryTestHarness(
        TemporaryApplicationPaths paths,
        DatabaseInitializer initializer,
        DatabaseWriteCoordinator writeCoordinator,
        FavoriteCategoryService categoryService,
        FavoriteChannelRepository channels,
        FavoriteUpdateRepository updates)
    {
        _paths = paths;
        _initializer = initializer;
        _writeCoordinator = writeCoordinator;
        Categories = categoryService;
        Channels = channels;
        Updates = updates;
    }

    public FavoriteCategoryService Categories { get; }
    public FavoriteChannelRepository Channels { get; }
    public FavoriteUpdateRepository Updates { get; }

    public static async Task<FavoriteRepositoryTestHarness> CreateAsync(
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
        var categoryRepository = new FavoriteCategoryRepository(connectionFactory, writeCoordinator);
        var categories = new FavoriteCategoryService(categoryRepository, new WeakReferenceMessenger());
        var channels = new FavoriteChannelRepository(connectionFactory, writeCoordinator);
        var updates = new FavoriteUpdateRepository(connectionFactory, writeCoordinator);
        return new FavoriteRepositoryTestHarness(
            paths, initializer, writeCoordinator, categories, channels, updates);
    }

    public async ValueTask DisposeAsync()
    {
        _initializer.Dispose();
        _writeCoordinator.Dispose();
        await _paths.DisposeAsync();
    }
}
