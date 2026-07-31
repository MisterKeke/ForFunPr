using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration011Bookmarks : IMigration
{
    public int Version => 11;

    public string Name => "create bookmarks and bookmark tags";

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
            CREATE TABLE IF NOT EXISTS bookmarks (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                url TEXT NOT NULL,
                url_normalized TEXT NOT NULL UNIQUE,
                title TEXT NOT NULL,
                description TEXT NOT NULL DEFAULT '',
                is_read INTEGER NOT NULL DEFAULT 0
                    CHECK (is_read IN (0, 1)),
                read_at DATETIME,
                revision INTEGER NOT NULL DEFAULT 1
                    CHECK (revision > 0),
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                CHECK (
                    length(CAST(url AS BLOB)) <= 4096
                        AND length(title) <= 200
                        AND length(CAST(description AS BLOB)) <= 16384
                )
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS bookmark_tags (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                name_normalized TEXT NOT NULL UNIQUE
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS bookmark_tag_assignments (
                bookmark_id INTEGER NOT NULL
                    REFERENCES bookmarks(id) ON DELETE CASCADE,
                tag_id INTEGER NOT NULL
                    REFERENCES bookmark_tags(id) ON DELETE CASCADE,
                PRIMARY KEY (bookmark_id, tag_id)
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_bookmarks_read_created
            ON bookmarks (is_read, created_at DESC, id DESC);
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_bookmark_tag_assignments_tag
            ON bookmark_tag_assignments (tag_id, bookmark_id);
            """);
    }
}
