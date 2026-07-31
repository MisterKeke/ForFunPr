using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration008TaskMetadata : IMigration
{
    public int Version => 8;

    public string Name => "add task difficulty tags and subtasks";

    public async Task UpAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken = default)
    {
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "todos",
            "difficulty",
            """
            ALTER TABLE todos ADD COLUMN difficulty TEXT
            CHECK (difficulty IS NULL OR difficulty IN ('easy', 'medium', 'hard'));
            """,
            cancellationToken);

        await MigrationSql.ExecuteStatementsAsync(
            connection,
            transaction,
            cancellationToken,
            """
            CREATE TABLE IF NOT EXISTS tags (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                name_normalized TEXT NOT NULL UNIQUE
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS todo_tags (
                todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
                tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
                PRIMARY KEY (todo_id, tag_id)
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS todo_subtasks (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
                title TEXT NOT NULL CHECK (TRIM(title) <> ''),
                is_completed INTEGER NOT NULL DEFAULT 0,
                position INTEGER NOT NULL DEFAULT 0,
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_todo_tags_tag
            ON todo_tags (tag_id);
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_todo_subtasks_todo_position
            ON todo_subtasks (todo_id, position, id);
            """);
    }
}
