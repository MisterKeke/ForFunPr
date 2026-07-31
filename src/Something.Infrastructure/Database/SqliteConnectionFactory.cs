using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;

namespace Something.Infrastructure.Database;

public sealed class SqliteConnectionFactory(IApplicationPaths applicationPaths)
    : IDatabaseConnectionFactory
{
    private static readonly string[] ConnectionPragmas =
    [
        "PRAGMA foreign_keys = ON;",
        "PRAGMA busy_timeout = 5000;",
        "PRAGMA journal_mode = WAL;",
    ];

    public async ValueTask<SqliteConnection> OpenConnectionAsync(
        CancellationToken cancellationToken = default)
    {
        var connectionString = new SqliteConnectionStringBuilder
        {
            DataSource = applicationPaths.DatabasePath,
            Mode = SqliteOpenMode.ReadWriteCreate,
            Cache = SqliteCacheMode.Shared,
            Pooling = true,
            DefaultTimeout = 5,
        }.ToString();

        var connection = new SqliteConnection(connectionString);

        try
        {
            await connection.OpenAsync(cancellationToken);

            foreach (var pragma in ConnectionPragmas)
            {
                await using var command = connection.CreateCommand();
                command.CommandText = pragma;
                await command.ExecuteNonQueryAsync(cancellationToken);
            }

            return connection;
        }
        catch
        {
            await connection.DisposeAsync();
            throw;
        }
    }
}
