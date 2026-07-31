using System.Globalization;
using System.Text.Json;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Domain.Enums;
using Something.Domain.Models.Favorites;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class FavoriteUpdateRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : IFavoriteUpdateRepository
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    public Task RecordApplicationOpenAsync(
        DateTimeOffset openedAt,
        CancellationToken cancellationToken = default)
    {
        openedAt = openedAt.ToUniversalTime();
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    var oldCurrent = openedAt;
                    await using (var read = CreateCommand(
                        connection,
                        transaction,
                        "SELECT value FROM app_state WHERE key = $key;",
                        ("$key", "current_opened_at")))
                    {
                        var value = await read.ExecuteScalarAsync(token);
                        if (value is string text && TryParseTimestamp(text, out var parsed))
                        {
                            oldCurrent = parsed;
                        }
                    }

                    await UpsertStateAsync(connection, transaction, "previous_opened_at", oldCurrent, token);
                    await UpsertStateAsync(connection, transaction, "current_opened_at", openedAt, token);
                    await InsertStateIfMissingAsync(connection, transaction, "previous_refresh_at", openedAt, token);
                    await InsertStateIfMissingAsync(connection, transaction, "last_refresh_at", openedAt, token);

                    await using (var prune = CreateCommand(
                        connection,
                        transaction,
                        "DELETE FROM favorite_news_items WHERE discovered_at < $cutoff;",
                        ("$cutoff", FormatTimestamp(openedAt.AddDays(-30)))))
                    {
                        await prune.ExecuteNonQueryAsync(token);
                    }

                    await using (var reset = CreateCommand(
                        connection,
                        transaction,
                        "DELETE FROM favorite_news_state WHERE id = 1;"))
                    {
                        await reset.ExecuteNonQueryAsync(token);
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

    public async Task<FavoriteUpdateState> GetStateAsync(CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await ReadStateAsync(connection, null, cancellationToken);
    }

    public async Task<IReadOnlyList<FavoriteUpdateSource>> ListSourcesAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = source switch
        {
            FavoriteSource.Telegram => "SELECT username, added_at FROM telegram_favorites ORDER BY added_at ASC;",
            FavoriteSource.YouTube => "SELECT channel_id, added_at FROM youtube_favorites ORDER BY added_at ASC;",
            _ => throw new ArgumentOutOfRangeException(nameof(source)),
        };

        var sources = new List<FavoriteUpdateSource>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            var sourceId = reader.GetString(0).Trim();
            if (sourceId.Length == 0)
            {
                continue;
            }

            var addedAt = TryParseTimestamp(reader.GetString(1), out var parsed)
                ? parsed
                : DateTimeOffset.MinValue;
            sources.Add(new FavoriteUpdateSource(source, sourceId, addedAt));
        }

        return sources;
    }

    public async Task<FavoriteSourceTracking> GetSourceTrackingAsync(
        FavoriteUpdateSource source,
        DateTimeOffset fallback,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        var sourceName = FavoriteRules.ToDatabase(source.Source);
        var checkedThrough = source.AddedAt > fallback ? source.AddedAt : fallback;

        await using (var checkpoint = CreateCommand(
            connection,
            null,
            """
            SELECT checked_through FROM favorite_update_checkpoints
            WHERE source = $source AND source_id = $sourceId;
            """,
            ("$source", sourceName),
            ("$sourceId", source.SourceId)))
        {
            var value = await checkpoint.ExecuteScalarAsync(cancellationToken);
            if (value is string text && TryParseTimestamp(text, out var parsed))
            {
                checkedThrough = parsed;
            }
        }

        await using var seen = CreateCommand(
            connection,
            null,
            """
            SELECT 1 FROM favorite_update_seen_items
            WHERE source = $source AND source_id = $sourceId LIMIT 1;
            """,
            ("$source", sourceName),
            ("$sourceId", source.SourceId));
        var hasSeen = await seen.ExecuteScalarAsync(cancellationToken) is not null;
        return new FavoriteSourceTracking(checkedThrough, hasSeen);
    }

    public Task<IReadOnlyList<FavoriteUpdateItem>> PersistSuccessfulSourceAsync(
        FavoriteUpdateSource source,
        FavoriteSourceTracking tracking,
        DateTimeOffset defaultCheckedThrough,
        DateTimeOffset scanStartedAt,
        IReadOnlyList<FavoriteUpdateItem> items,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync<IReadOnlyList<FavoriteUpdateItem>>(
            async token =>
            {
                var additions = new List<FavoriteUpdateItem>();
                var sourceName = FavoriteRules.ToDatabase(source.Source);
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    foreach (var item in items)
                    {
                        if (item.Source != source.Source ||
                            !string.Equals(item.SourceId, source.SourceId, StringComparison.Ordinal) ||
                            string.IsNullOrWhiteSpace(item.ItemId))
                        {
                            continue;
                        }

                        var isNew = false;
                        DateTimeOffset firstSeenAt;
                        await using (var readSeen = CreateCommand(
                            connection,
                            transaction,
                            """
                            SELECT first_seen_at FROM favorite_update_seen_items
                            WHERE source = $source AND source_id = $sourceId AND item_id = $itemId;
                            """,
                            ("$source", sourceName),
                            ("$sourceId", source.SourceId),
                            ("$itemId", item.ItemId)))
                        {
                            var value = await readSeen.ExecuteScalarAsync(token);
                            if (value is string text && TryParseTimestamp(text, out var parsed))
                            {
                                firstSeenAt = parsed;
                            }
                            else
                            {
                                isNew = true;
                                firstSeenAt = !tracking.HasSeenItems && item.PublishedAt <= tracking.CheckedThrough
                                    ? tracking.CheckedThrough
                                    : scanStartedAt;
                            }
                        }

                        if (isNew)
                        {
                            await using var insertSeen = CreateCommand(
                                connection,
                                transaction,
                                """
                                INSERT INTO favorite_update_seen_items (
                                    source, source_id, item_id, published_at, first_seen_at, updated_at
                                ) VALUES ($source, $sourceId, $itemId, $publishedAt, $firstSeenAt, CURRENT_TIMESTAMP)
                                ON CONFLICT(source, source_id, item_id) DO UPDATE SET
                                    published_at = excluded.published_at,
                                    updated_at = CURRENT_TIMESTAMP;
                                """,
                                ("$source", sourceName),
                                ("$sourceId", source.SourceId),
                                ("$itemId", item.ItemId),
                                ("$publishedAt", FormatTimestamp(item.PublishedAt)),
                                ("$firstSeenAt", FormatTimestamp(firstSeenAt)));
                            await insertSeen.ExecuteNonQueryAsync(token);
                        }

                        var payload = JsonSerializer.Serialize(item, JsonOptions);
                        if (isNew && firstSeenAt > defaultCheckedThrough && firstSeenAt <= scanStartedAt)
                        {
                            await using var upsertNews = CreateCommand(
                                connection,
                                transaction,
                                """
                                INSERT INTO favorite_news_items (
                                    source, source_id, item_id, published_at, discovered_at, payload_json, updated_at
                                ) VALUES ($source, $sourceId, $itemId, $publishedAt, $discoveredAt, $payload, CURRENT_TIMESTAMP)
                                ON CONFLICT(source, source_id, item_id) DO UPDATE SET
                                    published_at = excluded.published_at,
                                    payload_json = excluded.payload_json,
                                    updated_at = CURRENT_TIMESTAMP;
                                """,
                                ("$source", sourceName),
                                ("$sourceId", source.SourceId),
                                ("$itemId", item.ItemId),
                                ("$publishedAt", FormatTimestamp(item.PublishedAt)),
                                ("$discoveredAt", FormatTimestamp(scanStartedAt)),
                                ("$payload", payload));
                            await upsertNews.ExecuteNonQueryAsync(token);
                            additions.Add(item);
                        }
                        else
                        {
                            await using var updateNews = CreateCommand(
                                connection,
                                transaction,
                                """
                                UPDATE favorite_news_items
                                SET published_at = $publishedAt, payload_json = $payload, updated_at = CURRENT_TIMESTAMP
                                WHERE source = $source AND source_id = $sourceId AND item_id = $itemId;
                                """,
                                ("$publishedAt", FormatTimestamp(item.PublishedAt)),
                                ("$payload", payload),
                                ("$source", sourceName),
                                ("$sourceId", source.SourceId),
                                ("$itemId", item.ItemId));
                            await updateNews.ExecuteNonQueryAsync(token);
                        }
                    }

                    await using var success = CreateCommand(
                        connection,
                        transaction,
                        """
                        INSERT INTO favorite_update_checkpoints (
                            source, source_id, checked_through, last_success_at,
                            last_attempted_at, last_error, updated_at
                        ) VALUES ($source, $sourceId, $when, $when, $when, NULL, CURRENT_TIMESTAMP)
                        ON CONFLICT(source, source_id) DO UPDATE SET
                            checked_through = excluded.checked_through,
                            last_success_at = excluded.last_success_at,
                            last_attempted_at = excluded.last_attempted_at,
                            last_error = NULL,
                            updated_at = CURRENT_TIMESTAMP;
                        """,
                        ("$source", sourceName),
                        ("$sourceId", source.SourceId),
                        ("$when", FormatTimestamp(scanStartedAt)));
                    await success.ExecuteNonQueryAsync(token);
                    await transaction.CommitAsync(token);
                    return additions;
                }
                catch
                {
                    try { await transaction.RollbackAsync(CancellationToken.None); } catch { }
                    throw;
                }
            },
            cancellationToken);
    }

    public Task RecordSourceFailureAsync(
        FavoriteUpdateSource source,
        DateTimeOffset checkedThrough,
        DateTimeOffset attemptedAt,
        string error,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = CreateCommand(
                    connection,
                    null,
                    """
                    INSERT INTO favorite_update_checkpoints (
                        source, source_id, checked_through, last_attempted_at, last_error, updated_at
                    ) VALUES ($source, $sourceId, $checkedThrough, $attemptedAt, $error, CURRENT_TIMESTAMP)
                    ON CONFLICT(source, source_id) DO UPDATE SET
                        last_attempted_at = excluded.last_attempted_at,
                        last_error = excluded.last_error,
                        updated_at = CURRENT_TIMESTAMP;
                    """,
                    ("$source", FavoriteRules.ToDatabase(source.Source)),
                    ("$sourceId", source.SourceId),
                    ("$checkedThrough", FormatTimestamp(checkedThrough)),
                    ("$attemptedAt", FormatTimestamp(attemptedAt)),
                    ("$error", error));
                await command.ExecuteNonQueryAsync(token);
            },
            cancellationToken);
    }

    public Task CompleteScanAsync(
        DateTimeOffset scanStartedAt,
        IReadOnlyList<FavoriteUpdateError> errors,
        bool isRefresh,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    if (isRefresh)
                    {
                        var state = await ReadStateAsync(connection, transaction, token);
                        var previousRefresh = state.LastRefreshAt ?? state.CurrentOpenedAt ?? scanStartedAt;
                        await UpsertStateAsync(connection, transaction, "previous_refresh_at", previousRefresh, token);
                        await UpsertStateAsync(connection, transaction, "last_refresh_at", scanStartedAt, token);
                    }

                    await using var scan = CreateCommand(
                        connection,
                        transaction,
                        """
                        INSERT INTO favorite_news_state (id, scan_started_at, errors_json, updated_at)
                        VALUES (1, $scanStartedAt, $errors, CURRENT_TIMESTAMP)
                        ON CONFLICT(id) DO UPDATE SET
                            scan_started_at = excluded.scan_started_at,
                            errors_json = excluded.errors_json,
                            updated_at = CURRENT_TIMESTAMP;
                        """,
                        ("$scanStartedAt", FormatTimestamp(scanStartedAt)),
                        ("$errors", JsonSerializer.Serialize(errors, JsonOptions)));
                    await scan.ExecuteNonQueryAsync(token);
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

    public async Task<FavoriteUpdateScanResult> GetCurrentAsync(CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        var state = await ReadStateAsync(connection, null, cancellationToken);
        DateTimeOffset? scanStartedAt = null;
        IReadOnlyList<FavoriteUpdateError> errors = [];

        await using (var stateCommand = CreateCommand(
            connection,
            null,
            "SELECT scan_started_at, errors_json FROM favorite_news_state WHERE id = 1;"))
        await using (var reader = await stateCommand.ExecuteReaderAsync(cancellationToken))
        {
            if (await reader.ReadAsync(cancellationToken))
            {
                if (TryParseTimestamp(reader.GetString(0), out var parsed))
                {
                    scanStartedAt = parsed;
                }

                errors = JsonSerializer.Deserialize<FavoriteUpdateError[]>(reader.GetString(1), JsonOptions) ?? [];
            }
        }

        if (scanStartedAt is null)
        {
            return new FavoriteUpdateScanResult(null, [], [], errors, state);
        }

        await using var news = CreateCommand(
            connection,
            null,
            """
            SELECT payload_json FROM favorite_news_items
            WHERE discovered_at = $scanStartedAt
            ORDER BY published_at DESC, discovered_at DESC;
            """,
            ("$scanStartedAt", FormatTimestamp(scanStartedAt.Value)));
        var updates = new List<FavoriteUpdateItem>();
        await using var newsReader = await news.ExecuteReaderAsync(cancellationToken);
        while (await newsReader.ReadAsync(cancellationToken))
        {
            var item = JsonSerializer.Deserialize<FavoriteUpdateItem>(newsReader.GetString(0), JsonOptions);
            if (item is not null)
            {
                updates.Add(item);
            }
        }

        return new FavoriteUpdateScanResult(scanStartedAt, updates, [], errors, state);
    }

    private static async Task<FavoriteUpdateState> ReadStateAsync(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        CancellationToken cancellationToken)
    {
        await using var command = CreateCommand(
            connection,
            transaction,
            """
            SELECT key, value FROM app_state
            WHERE key IN ('previous_opened_at', 'current_opened_at', 'previous_refresh_at', 'last_refresh_at');
            """);
        var values = new Dictionary<string, DateTimeOffset>(StringComparer.Ordinal);
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            if (TryParseTimestamp(reader.GetString(1), out var timestamp))
            {
                values[reader.GetString(0)] = timestamp;
            }
        }

        return new FavoriteUpdateState(
            GetValue(values, "previous_opened_at"),
            GetValue(values, "current_opened_at"),
            GetValue(values, "previous_refresh_at"),
            GetValue(values, "last_refresh_at"));
    }

    private static async Task UpsertStateAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string key,
        DateTimeOffset value,
        CancellationToken cancellationToken)
    {
        await using var command = CreateCommand(
            connection,
            transaction,
            """
            INSERT INTO app_state (key, value) VALUES ($key, $value)
            ON CONFLICT(key) DO UPDATE SET value = excluded.value;
            """,
            ("$key", key),
            ("$value", FormatTimestamp(value)));
        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static async Task InsertStateIfMissingAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string key,
        DateTimeOffset value,
        CancellationToken cancellationToken)
    {
        await using var command = CreateCommand(
            connection,
            transaction,
            "INSERT OR IGNORE INTO app_state (key, value) VALUES ($key, $value);",
            ("$key", key),
            ("$value", FormatTimestamp(value)));
        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static SqliteCommand CreateCommand(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        string text,
        params (string Name, object? Value)[] parameters)
    {
        var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = text;
        foreach (var parameter in parameters)
        {
            command.Parameters.AddWithValue(parameter.Name, parameter.Value ?? DBNull.Value);
        }

        return command;
    }

    private static DateTimeOffset? GetValue(
        IReadOnlyDictionary<string, DateTimeOffset> values,
        string key) => values.TryGetValue(key, out var value) ? value : null;

    private static string FormatTimestamp(DateTimeOffset value) =>
        value.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture);

    private static bool TryParseTimestamp(string value, out DateTimeOffset timestamp) =>
        DateTimeOffset.TryParse(
            value,
            CultureInfo.InvariantCulture,
            DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal,
            out timestamp);
}
