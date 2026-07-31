using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database;

public interface IMigration
{
    int Version { get; }

    string Name { get; }

    Task UpAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken = default);
}
