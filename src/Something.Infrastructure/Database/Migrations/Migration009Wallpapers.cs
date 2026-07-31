using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration009Wallpapers : IMigration
{
    public int Version => 9;

    public string Name => "create user wallpaper metadata";

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
            CREATE TABLE IF NOT EXISTS user_wallpapers (
                id TEXT PRIMARY KEY,
                display_name TEXT NOT NULL,
                filename TEXT NOT NULL UNIQUE,
                mime_type TEXT NOT NULL
                    CHECK(mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
                byte_size INTEGER NOT NULL CHECK(byte_size > 0),
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """);
    }
}
