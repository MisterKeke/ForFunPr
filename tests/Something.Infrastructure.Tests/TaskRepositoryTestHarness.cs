using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Features.Tasks;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

internal sealed class TaskRepositoryTestHarness : IAsyncDisposable
{
    private readonly TemporaryApplicationPaths _paths;
    private readonly DatabaseInitializer _initializer;
    private readonly DatabaseWriteCoordinator _writeCoordinator;

    private TaskRepositoryTestHarness(
        TemporaryApplicationPaths paths,
        SqliteConnectionFactory connectionFactory,
        DatabaseInitializer initializer,
        DatabaseWriteCoordinator writeCoordinator,
        TaskRepository repository,
        TaskService service)
    {
        _paths = paths;
        ConnectionFactory = connectionFactory;
        _initializer = initializer;
        _writeCoordinator = writeCoordinator;
        Repository = repository;
        Service = service;
    }

    public SqliteConnectionFactory ConnectionFactory { get; }
    public TaskRepository Repository { get; }
    public TaskService Service { get; }

    public static async Task<TaskRepositoryTestHarness> CreateAsync()
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
        await initializer.InitializeAsync();

        var writeCoordinator = new DatabaseWriteCoordinator();
        var repository = new TaskRepository(connectionFactory, writeCoordinator);
        var service = new TaskService(repository, new WeakReferenceMessenger());
        return new TaskRepositoryTestHarness(
            paths, connectionFactory, initializer, writeCoordinator, repository, service);
    }

    public async ValueTask DisposeAsync()
    {
        _initializer.Dispose();
        _writeCoordinator.Dispose();
        await _paths.DisposeAsync();
    }
}
