using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration001CoreSchema : IMigration
{
    public int Version => 1;

    public string Name => "create core currency and todo tables";

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
            CREATE TABLE IF NOT EXISTS favorite_rates (
                base TEXT NOT NULL,
                quote TEXT NOT NULL,
                PRIMARY KEY (base, quote)
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS todos (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT NOT NULL,
                description TEXT,
                is_completed INTEGER NOT NULL DEFAULT 0,
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                due_date DATE,
                priority TEXT NOT NULL DEFAULT 'medium'
                    CHECK(priority IN ('low', 'medium', 'high'))
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_todos_due_incomplete_priority_created_at
            ON todos (due_date, is_completed, priority, created_at);
            """);
    }
}
