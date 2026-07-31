using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Features.Notes;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

internal sealed class NoteRepositoryTestHarness : IAsyncDisposable
{
    private readonly TemporaryApplicationPaths _paths;
    private readonly DatabaseInitializer _initializer;
    private readonly DatabaseWriteCoordinator _writeCoordinator;

    private NoteRepositoryTestHarness(
        TemporaryApplicationPaths paths,
        DatabaseInitializer initializer,
        DatabaseWriteCoordinator writeCoordinator,
        NoteService service)
    {
        _paths = paths;
        _initializer = initializer;
        _writeCoordinator = writeCoordinator;
        Service = service;
    }

    public NoteService Service { get; }

    public static async Task<NoteRepositoryTestHarness> CreateAsync(CancellationToken cancellationToken)
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
        var repository = new NoteRepository(connectionFactory, writeCoordinator);
        var service = new NoteService(repository, new WeakReferenceMessenger());
        return new NoteRepositoryTestHarness(paths, initializer, writeCoordinator, service);
    }

    public async ValueTask DisposeAsync()
    {
        _initializer.Dispose();
        _writeCoordinator.Dispose();
        await _paths.DisposeAsync();
    }
}
