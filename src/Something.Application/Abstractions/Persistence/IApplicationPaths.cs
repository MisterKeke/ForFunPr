namespace Something.Application.Abstractions.Persistence;

public interface IApplicationPaths
{
    string DataDirectory { get; }

    string DatabasePath { get; }

    string UserWallpapersDirectory { get; }

    string LogsDirectory { get; }

    Task EnsureDirectoriesAsync(CancellationToken cancellationToken = default);
}
