using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration007FavoriteNews : IMigration
{
    public int Version => 7;

    public string Name => "persist favorite news scan results";

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
            CREATE TABLE IF NOT EXISTS favorite_news_items (
                source TEXT NOT NULL,
                source_id TEXT NOT NULL,
                item_id TEXT NOT NULL,
                published_at TEXT NOT NULL,
                discovered_at TEXT NOT NULL,
                payload_json TEXT NOT NULL,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (source, source_id, item_id)
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_favorite_news_items_discovered
            ON favorite_news_items (discovered_at, published_at);
            """,
            """
            CREATE TABLE IF NOT EXISTS favorite_news_state (
                id INTEGER PRIMARY KEY CHECK (id = 1),
                scan_started_at TEXT NOT NULL,
                errors_json TEXT NOT NULL DEFAULT '[]',
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """);
    }
}
