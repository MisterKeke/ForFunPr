using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;

namespace Something.Infrastructure.Tests;

internal sealed class TemporaryApplicationPaths : IApplicationPaths, IAsyncDisposable
{
    private TemporaryApplicationPaths(string dataDirectory)
    {
        DataDirectory = dataDirectory;
        DatabasePath = Path.Combine(dataDirectory, "database.db");
        UserWallpapersDirectory = Path.Combine(dataDirectory, "user-wallpapers");
        LogsDirectory = Path.Combine(dataDirectory, "logs");
    }

    public string DataDirectory { get; }
    public string DatabasePath { get; }
    public string UserWallpapersDirectory { get; }
    public string LogsDirectory { get; }

    public static TemporaryApplicationPaths Create()
    {
        var root = Path.Combine(
            Path.GetTempPath(),
            "SomethingCSharp.Tests",
            Guid.NewGuid().ToString("N"));
        return new TemporaryApplicationPaths(root);
    }

    public Task EnsureDirectoriesAsync(CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        Directory.CreateDirectory(DataDirectory);
        Directory.CreateDirectory(UserWallpapersDirectory);
        Directory.CreateDirectory(LogsDirectory);
        return Task.CompletedTask;
    }

    public ValueTask DisposeAsync()
    {
        SqliteConnection.ClearAllPools();

        if (Directory.Exists(DataDirectory))
        {
            Directory.Delete(DataDirectory, recursive: true);
        }

        return ValueTask.CompletedTask;
    }
}
