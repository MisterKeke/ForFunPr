using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration010Notes : IMigration
{
    public int Version => 10;

    public string Name => "create notes";

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
            CREATE TABLE IF NOT EXISTS notes (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT NOT NULL DEFAULT '',
                body TEXT NOT NULL DEFAULT '',
                is_pinned INTEGER NOT NULL DEFAULT 0
                    CHECK (is_pinned IN (0, 1)),
                is_archived INTEGER NOT NULL DEFAULT 0
                    CHECK (is_archived IN (0, 1)),
                revision INTEGER NOT NULL DEFAULT 1
                    CHECK (revision > 0),
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                CHECK (
                    length(title) <= 200
                        AND length(CAST(body AS BLOB)) <= 262144
                )
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_notes_archive_pin_updated
            ON notes (is_archived, is_pinned DESC, updated_at DESC, id DESC);
            """);
    }
}
