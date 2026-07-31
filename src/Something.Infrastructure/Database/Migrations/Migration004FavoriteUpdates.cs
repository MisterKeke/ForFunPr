using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration004FavoriteUpdates : IMigration
{
    public int Version => 4;

    public string Name => "create favorite update tracking tables";

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
            CREATE TABLE IF NOT EXISTS favorite_update_checkpoints (
                source TEXT NOT NULL,
                source_id TEXT NOT NULL,
                checked_through TEXT NOT NULL,
                last_success_at TEXT,
                last_attempted_at TEXT,
                last_error TEXT,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (source, source_id)
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS favorite_update_seen_items (
                source TEXT NOT NULL,
                source_id TEXT NOT NULL,
                item_id TEXT NOT NULL,
                published_at TEXT NOT NULL,
                first_seen_at TEXT NOT NULL,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (source, source_id, item_id)
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_favorite_update_seen_items_source_published
            ON favorite_update_seen_items (source, source_id, published_at);
            """);
    }
}
