using System.Globalization;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Favorites;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class FavoriteCategoryRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : IFavoriteCategoryRepository
{
    public async Task<IReadOnlyList<FavoriteCategory>> ListAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            SELECT id, name, source, COALESCE(color, ''), created_at
            FROM favorite_categories
            WHERE source = $source
            ORDER BY LOWER(name) ASC;
            """;
        command.Parameters.AddWithValue("$source", FavoriteRules.ToDatabase(source));
        var categories = new List<FavoriteCategory>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            categories.Add(ReadCategory(reader));
        }

        return categories;
    }

    public Task<(FavoriteCategory Category, bool Created)> CreateAsync(
        string name,
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    INSERT INTO favorite_categories (name, name_normalized, source)
                    VALUES ($name, $normalized, $source)
                    ON CONFLICT(name_normalized) DO NOTHING
                    RETURNING id, name, source, COALESCE(color, ''), created_at;
                    """;
                command.Parameters.AddWithValue("$name", name);
                command.Parameters.AddWithValue("$normalized", FavoriteRules.CategoryKey(source, name));
                command.Parameters.AddWithValue("$source", FavoriteRules.ToDatabase(source));
                await using (var reader = await command.ExecuteReaderAsync(token))
                {
                    if (await reader.ReadAsync(token))
                    {
                        return (ReadCategory(reader), true);
                    }
                }

                await using var existing = connection.CreateCommand();
                existing.CommandText = """
                    SELECT id, name, source, COALESCE(color, ''), created_at
                    FROM favorite_categories
                    WHERE name_normalized = $normalized;
                    """;
                existing.Parameters.AddWithValue("$normalized", FavoriteRules.CategoryKey(source, name));
                await using var existingReader = await existing.ExecuteReaderAsync(token);
                if (!await existingReader.ReadAsync(token))
                {
                    throw new InvalidOperationException("Existing favorite category could not be loaded.");
                }

                return (ReadCategory(existingReader), false);
            },
            cancellationToken);
    }

    public Task<FavoriteCategory> RenameAsync(
        long id,
        string name,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    FavoriteSource source;
                    await using (var sourceCommand = connection.CreateCommand())
                    {
                        sourceCommand.Transaction = transaction;
                        sourceCommand.CommandText = "SELECT source FROM favorite_categories WHERE id = $id;";
                        sourceCommand.Parameters.AddWithValue("$id", id);
                        var value = await sourceCommand.ExecuteScalarAsync(token);
                        if (value is null)
                        {
                            throw new NotFoundException($"Favorite category {id} was not found.");
                        }

                        source = FavoriteRules.ParseSource(Convert.ToString(value, CultureInfo.InvariantCulture)!);
                    }

                    var normalized = FavoriteRules.CategoryKey(source, name);
                    await using (var conflictCommand = connection.CreateCommand())
                    {
                        conflictCommand.Transaction = transaction;
                        conflictCommand.CommandText = """
                            SELECT id FROM favorite_categories
                            WHERE name_normalized = $normalized AND id <> $id
                            LIMIT 1;
                            """;
                        conflictCommand.Parameters.AddWithValue("$normalized", normalized);
                        conflictCommand.Parameters.AddWithValue("$id", id);
                        if (await conflictCommand.ExecuteScalarAsync(token) is not null)
                        {
                            throw new ConflictException(
                                "A favorite category with that name already exists for this source.");
                        }
                    }

                    FavoriteCategory category;
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            UPDATE favorite_categories
                            SET name = $name, name_normalized = $normalized
                            WHERE id = $id
                            RETURNING id, name, source, COALESCE(color, ''), created_at;
                            """;
                        command.Parameters.AddWithValue("$name", name);
                        command.Parameters.AddWithValue("$normalized", normalized);
                        command.Parameters.AddWithValue("$id", id);
                        await using var reader = await command.ExecuteReaderAsync(token);
                        if (!await reader.ReadAsync(token))
                        {
                            throw new NotFoundException($"Favorite category {id} was not found.");
                        }

                        category = ReadCategory(reader);
                    }

                    await transaction.CommitAsync(token);
                    return category;
                }
                catch
                {
                    try { await transaction.RollbackAsync(CancellationToken.None); } catch { }
                    throw;
                }
            },
            cancellationToken);
    }

    public async Task<bool> ExistsAsync(
        long id,
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            SELECT COUNT(*) FROM favorite_categories
            WHERE id = $id AND source = $source;
            """;
        command.Parameters.AddWithValue("$id", id);
        command.Parameters.AddWithValue("$source", FavoriteRules.ToDatabase(source));
        return Convert.ToInt32(await command.ExecuteScalarAsync(cancellationToken)) == 1;
    }

    private static FavoriteCategory ReadCategory(SqliteDataReader reader)
    {
        return new FavoriteCategory(
            reader.GetInt64(0),
            reader.GetString(1),
            FavoriteRules.ParseSource(reader.GetString(2)),
            reader.GetString(3),
            ParseTimestamp(reader.GetString(4)));
    }

    private static DateTimeOffset ParseTimestamp(string value)
    {
        return DateTimeOffset.TryParse(
            value,
            CultureInfo.InvariantCulture,
            DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal,
            out var timestamp)
            ? timestamp
            : DateTimeOffset.MinValue;
    }
}
