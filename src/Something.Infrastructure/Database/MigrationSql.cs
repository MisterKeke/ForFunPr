using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database;

internal static class MigrationSql
{
    public static async Task ExecuteStatementsAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken,
        params string[] statements)
    {
        foreach (var statement in statements)
        {
            await ExecuteAsync(
                connection,
                transaction,
                statement,
                cancellationToken);
        }
    }

    public static async Task ExecuteAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string commandText,
        CancellationToken cancellationToken,
        params (string Name, object? Value)[] parameters)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = commandText;

        foreach (var (name, value) in parameters)
        {
            command.Parameters.AddWithValue(name, value ?? DBNull.Value);
        }

        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    public static async Task<bool> TableHasColumnAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string table,
        string column,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = """
            SELECT EXISTS (
                SELECT 1
                FROM pragma_table_info($table)
                WHERE name = $column
            );
            """;
        command.Parameters.AddWithValue("$table", table);
        command.Parameters.AddWithValue("$column", column);

        var result = await command.ExecuteScalarAsync(cancellationToken);
        return Convert.ToInt32(result) != 0;
    }

    public static async Task AddColumnIfMissingAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string table,
        string column,
        string alterTableStatement,
        CancellationToken cancellationToken)
    {
        if (await TableHasColumnAsync(
                connection,
                transaction,
                table,
                column,
                cancellationToken))
        {
            return;
        }

        await ExecuteAsync(
            connection,
            transaction,
            alterTableStatement,
            cancellationToken);
    }
}
