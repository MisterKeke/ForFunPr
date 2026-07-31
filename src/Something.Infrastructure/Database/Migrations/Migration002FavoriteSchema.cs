using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration002FavoriteSchema : IMigration
{
    public int Version => 2;

    public string Name => "create favorite categories and source favorites";

    public async Task UpAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken = default)
    {
        await MigrationSql.ExecuteStatementsAsync(
            connection,
            transaction,
            cancellationToken,
            """
            CREATE TABLE IF NOT EXISTS favorite_categories (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                name_normalized TEXT NOT NULL,
                source TEXT NOT NULL DEFAULT 'telegram',
                color TEXT,
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS telegram_favorites (
                username TEXT PRIMARY KEY,
                added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL
            );
            """,
            """
            CREATE TABLE IF NOT EXISTS youtube_favorites (
                channel_id TEXT PRIMARY KEY,
                added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL,
                username TEXT
            );
            """);

        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "favorite_categories",
            "source",
            "ALTER TABLE favorite_categories ADD COLUMN source TEXT NOT NULL DEFAULT 'telegram';",
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "favorite_categories",
            "color",
            "ALTER TABLE favorite_categories ADD COLUMN color TEXT;",
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "favorite_categories",
            "name_normalized",
            "ALTER TABLE favorite_categories ADD COLUMN name_normalized TEXT;",
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "telegram_favorites",
            "category_id",
            """
            ALTER TABLE telegram_favorites
            ADD COLUMN category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL;
            """,
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "youtube_favorites",
            "category_id",
            """
            ALTER TABLE youtube_favorites
            ADD COLUMN category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL;
            """,
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "youtube_favorites",
            "username",
            "ALTER TABLE youtube_favorites ADD COLUMN username TEXT;",
            cancellationToken);

        await MigrationSql.ExecuteAsync(
            connection,
            transaction,
            """
            UPDATE favorite_categories
            SET source = CASE
                WHEN LOWER(TRIM(source)) = 'youtube' THEN 'youtube'
                ELSE 'telegram'
            END;
            """,
            cancellationToken);
        await MigrationSql.ExecuteAsync(
            connection,
            transaction,
            """
            UPDATE favorite_categories
            SET name_normalized = LOWER(TRIM(source)) || ':' || LOWER(TRIM(name))
            WHERE name_normalized IS NULL OR TRIM(name_normalized) = '';
            """,
            cancellationToken);

        await EnsureUniqueCategoryKeyAsync(
            connection,
            transaction,
            cancellationToken);

        await MigrationSql.ExecuteStatementsAsync(
            connection,
            transaction,
            cancellationToken,
            """
            CREATE INDEX IF NOT EXISTS idx_favorite_categories_source_name
            ON favorite_categories (source, name_normalized);
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_telegram_favorites_category
            ON telegram_favorites (category_id);
            """,
            """
            CREATE INDEX IF NOT EXISTS idx_youtube_favorites_category
            ON youtube_favorites (category_id);
            """);
    }

    private static async Task EnsureUniqueCategoryKeyAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken)
    {
        await using var duplicateCommand = connection.CreateCommand();
        duplicateCommand.Transaction = transaction;
        duplicateCommand.CommandText = """
            SELECT COUNT(*)
            FROM (
                SELECT name_normalized
                FROM favorite_categories
                GROUP BY name_normalized
                HAVING COUNT(*) > 1
            );
            """;

        var duplicateResult = await duplicateCommand.ExecuteScalarAsync(cancellationToken);
        var duplicateCount = Convert.ToInt32(duplicateResult);
        if (duplicateCount > 0)
        {
            throw new InvalidOperationException(
                $"Cannot migrate favorite categories: {duplicateCount} duplicate normalized name(s) require review.");
        }

        if (await HasUniqueSingleColumnIndexAsync(
                connection,
                transaction,
                "favorite_categories",
                "name_normalized",
                cancellationToken))
        {
            return;
        }

        await MigrationSql.ExecuteAsync(
            connection,
            transaction,
            """
            CREATE UNIQUE INDEX IF NOT EXISTS ux_favorite_categories_name_normalized
            ON favorite_categories (name_normalized);
            """,
            cancellationToken);
    }

    private static async Task<bool> HasUniqueSingleColumnIndexAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string table,
        string column,
        CancellationToken cancellationToken)
    {
        var uniqueIndexNames = new List<string>();

        await using (var indexCommand = connection.CreateCommand())
        {
            indexCommand.Transaction = transaction;
            indexCommand.CommandText = """
                SELECT name
                FROM pragma_index_list($table)
                WHERE "unique" <> 0;
                """;
            indexCommand.Parameters.AddWithValue("$table", table);

            await using var reader = await indexCommand.ExecuteReaderAsync(cancellationToken);
            while (await reader.ReadAsync(cancellationToken))
            {
                uniqueIndexNames.Add(reader.GetString(0));
            }
        }

        foreach (var indexName in uniqueIndexNames)
        {
            await using var columnCommand = connection.CreateCommand();
            columnCommand.Transaction = transaction;
            columnCommand.CommandText = """
                SELECT name
                FROM pragma_index_info($indexName);
                """;
            columnCommand.Parameters.AddWithValue("$indexName", indexName);

            var columnCount = 0;
            var matchesColumn = true;
            await using var reader = await columnCommand.ExecuteReaderAsync(cancellationToken);
            while (await reader.ReadAsync(cancellationToken))
            {
                columnCount++;
                matchesColumn &= !reader.IsDBNull(0) && reader.GetString(0) == column;
            }

            if (columnCount == 1 && matchesColumn)
            {
                return true;
            }
        }

        return false;
    }
}
