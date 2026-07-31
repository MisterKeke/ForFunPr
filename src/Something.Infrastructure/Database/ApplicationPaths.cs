using Something.Application.Abstractions.Persistence;

namespace Something.Infrastructure.Database;

public sealed class ApplicationPaths : IApplicationPaths
{
    private const string DevelopmentDirectoryName = "Something.CSharp.Dev";
    private const string DatabaseFileName = "database.db";
    private const string UserWallpapersDirectoryName = "user-wallpapers";
    private const string LogsDirectoryName = "logs";
    private const string DevelopmentDataOverride = "SOMETHING_CSHARP_DEV_DATA";

    public ApplicationPaths()
    {
        var configuredDirectory = Environment.GetEnvironmentVariable(DevelopmentDataOverride);
        if (!string.IsNullOrWhiteSpace(configuredDirectory))
        {
            DataDirectory = Path.GetFullPath(configuredDirectory.Trim());
            DatabasePath = Path.Combine(DataDirectory, DatabaseFileName);
            UserWallpapersDirectory = Path.Combine(DataDirectory, UserWallpapersDirectoryName);
            LogsDirectory = Path.Combine(DataDirectory, LogsDirectoryName);
            return;
        }

        var roamingApplicationData = Environment.GetFolderPath(
            Environment.SpecialFolder.ApplicationData,
            Environment.SpecialFolderOption.DoNotVerify);

        if (string.IsNullOrWhiteSpace(roamingApplicationData))
        {
            throw new InvalidOperationException("The application-data directory could not be resolved.");
        }

        DataDirectory = Path.Combine(roamingApplicationData, DevelopmentDirectoryName);
        DatabasePath = Path.Combine(DataDirectory, DatabaseFileName);
        UserWallpapersDirectory = Path.Combine(DataDirectory, UserWallpapersDirectoryName);
        LogsDirectory = Path.Combine(DataDirectory, LogsDirectoryName);
    }

    public string DataDirectory { get; }

    public string DatabasePath { get; }

    public string UserWallpapersDirectory { get; }

    public string LogsDirectory { get; }

    public Task EnsureDirectoriesAsync(CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                Directory.CreateDirectory(DataDirectory);
                Directory.CreateDirectory(UserWallpapersDirectory);
                Directory.CreateDirectory(LogsDirectory);
            },
            cancellationToken);
    }
}
