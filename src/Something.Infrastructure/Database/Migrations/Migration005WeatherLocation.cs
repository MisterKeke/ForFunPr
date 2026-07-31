using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration005WeatherLocation : IMigration
{
    public int Version => 5;

    public string Name => "create saved weather location table";

    public Task UpAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken = default)
    {
        return MigrationSql.ExecuteStatementsAsync(
            connection,
            transaction,
            cancellationToken,
            """
            CREATE TABLE IF NOT EXISTS location (
                id INTEGER PRIMARY KEY CHECK (id = 1),
                latitude REAL NOT NULL CHECK (latitude >= -90 AND latitude <= 90),
                longitude REAL NOT NULL CHECK (longitude >= -180 AND longitude <= 180),
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """);
    }
}
