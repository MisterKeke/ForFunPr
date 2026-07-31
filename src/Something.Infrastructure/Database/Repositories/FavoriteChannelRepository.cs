using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Favorites;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class FavoriteChannelRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : IFavoriteChannelRepository
{
    public Task AddTelegramAsync(string username, CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = "INSERT OR IGNORE INTO telegram_favorites (username) VALUES ($username);";
                command.Parameters.AddWithValue("$username", username);
                await command.ExecuteNonQueryAsync(token);
            },
            cancellationToken);
    }

    public Task AddYouTubeAsync(
        string channelId,
        string handle,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    INSERT INTO youtube_favorites (channel_id, username)
                    VALUES ($channelId, NULLIF($handle, ''))
                    ON CONFLICT(channel_id) DO UPDATE SET
                        username = COALESCE(NULLIF(excluded.username, ''), youtube_favorites.username);
                    """;
                command.Parameters.AddWithValue("$channelId", channelId);
                command.Parameters.AddWithValue("$handle", handle);
                await command.ExecuteNonQueryAsync(token);
            },
            cancellationToken);
    }

    public Task RemoveAsync(
        FavoriteSource source,
        string sourceId,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = source switch
                {
                    FavoriteSource.Telegram => "DELETE FROM telegram_favorites WHERE username = $sourceId;",
                    FavoriteSource.YouTube => "DELETE FROM youtube_favorites WHERE channel_id = $sourceId;",
                    _ => throw new ValidationException("Source must be Telegram or YouTube."),
                };
                command.Parameters.AddWithValue("$sourceId", sourceId);
                await command.ExecuteNonQueryAsync(token);
            },
            cancellationToken);
    }

    public Task AssignCategoryAsync(
        FavoriteSource source,
        string sourceId,
        long categoryId,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    await using (var categoryCommand = connection.CreateCommand())
                    {
                        categoryCommand.Transaction = transaction;
                        categoryCommand.CommandText = """
                            SELECT COUNT(*) FROM favorite_categories
                            WHERE id = $categoryId AND source = $source;
                            """;
                        categoryCommand.Parameters.AddWithValue("$categoryId", categoryId);
                        categoryCommand.Parameters.AddWithValue("$source", FavoriteRules.ToDatabase(source));
                        if (Convert.ToInt32(await categoryCommand.ExecuteScalarAsync(token)) != 1)
                        {
                            throw new NotFoundException($"Favorite category {categoryId} was not found.");
                        }
                    }

                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = source switch
                        {
                            FavoriteSource.Telegram => """
                                UPDATE telegram_favorites SET category_id = $categoryId
                                WHERE username = $sourceId;
                                """,
                            FavoriteSource.YouTube => """
                                UPDATE youtube_favorites SET category_id = $categoryId
                                WHERE channel_id = $sourceId;
                                """,
                            _ => throw new ValidationException("Source must be Telegram or YouTube."),
                        };
                        command.Parameters.AddWithValue("$categoryId", categoryId);
                        command.Parameters.AddWithValue("$sourceId", sourceId);
                        if (await command.ExecuteNonQueryAsync(token) != 1)
                        {
                            throw new NotFoundException($"Favorite {sourceId} was not found.");
                        }
                    }

                    await transaction.CommitAsync(token);
                }
                catch
                {
                    try { await transaction.RollbackAsync(CancellationToken.None); } catch { }
                    throw;
                }
            },
            cancellationToken);
    }

    public async Task<IReadOnlyList<FavoriteChannel>> ListAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = source switch
        {
            FavoriteSource.Telegram => """
                SELECT username, username, category_id
                FROM telegram_favorites ORDER BY added_at ASC;
                """,
            FavoriteSource.YouTube => """
                SELECT channel_id, COALESCE(NULLIF(username, ''), channel_id), category_id
                FROM youtube_favorites ORDER BY added_at ASC;
                """,
            _ => throw new ValidationException("Source must be Telegram or YouTube."),
        };
        var channels = new List<FavoriteChannel>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            channels.Add(new FavoriteChannel(
                reader.GetString(0),
                reader.GetString(1),
                source,
                reader.IsDBNull(2) ? null : reader.GetInt64(2)));
        }

        return channels;
    }
}
