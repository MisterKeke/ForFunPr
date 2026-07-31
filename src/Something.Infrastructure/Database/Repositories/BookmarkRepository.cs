using System.Globalization;
using System.Text;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Application.Features.Bookmarks;
using Something.Domain.Exceptions;
using Something.Domain.Models.Bookmarks;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class BookmarkRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : IBookmarkRepository
{
    public async Task<BookmarkListResult> ListAsync(
        BookmarkFilter filter,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        var sql = new StringBuilder("1 = 1");
        await using var command = connection.CreateCommand();

        switch (filter.Status)
        {
            case BookmarkReadStatus.Read:
                sql.Append(" AND b.is_read = 1");
                break;
            case BookmarkReadStatus.Unread:
                sql.Append(" AND b.is_read = 0");
                break;
            case BookmarkReadStatus.All:
                break;
            default:
                throw new ValidationException("Unknown bookmark status filter.");
        }

        if (filter.Query.Length > 0)
        {
            sql.Append("""
                 AND (
                    LOWER(b.title) LIKE $query ESCAPE '\'
                    OR LOWER(b.url) LIKE $query ESCAPE '\'
                    OR LOWER(b.description) LIKE $query ESCAPE '\'
                    OR EXISTS (
                        SELECT 1
                        FROM bookmark_tag_assignments AS search_bta
                        JOIN bookmark_tags AS search_bt ON search_bt.id = search_bta.tag_id
                        WHERE search_bta.bookmark_id = b.id
                          AND search_bt.name_normalized LIKE $query ESCAPE '\'
                    )
                )
                """);
            command.Parameters.AddWithValue("$query", ToLikePattern(filter.Query));
        }

        for (var index = 0; index < filter.Tags.Count; index++)
        {
            var parameter = $"$tag{index}";
            sql.Append($$"""
                 AND EXISTS (
                    SELECT 1
                    FROM bookmark_tag_assignments AS filter_bta
                    JOIN bookmark_tags AS filter_bt ON filter_bt.id = filter_bta.tag_id
                    WHERE filter_bta.bookmark_id = b.id
                      AND filter_bt.name_normalized = {{parameter}}
                )
                """);
            command.Parameters.AddWithValue(parameter, filter.Tags[index].ToLowerInvariant());
        }

        var whereSql = sql.ToString();
        int total;
        await using (var countCommand = connection.CreateCommand())
        {
            countCommand.CommandText = $"SELECT COUNT(*) FROM bookmarks AS b WHERE {whereSql};";
            CopyParameters(command, countCommand);
            total = Convert.ToInt32(await countCommand.ExecuteScalarAsync(cancellationToken));
        }

        command.CommandText = $"""
            SELECT b.id, b.url, b.title, b.description, b.is_read, b.read_at,
                   b.revision, b.created_at, b.updated_at
            FROM bookmarks AS b
            WHERE {whereSql}
            ORDER BY b.is_read ASC, b.created_at DESC, b.id DESC
            LIMIT $limit OFFSET $offset;
            """;
        command.Parameters.AddWithValue("$limit", filter.Limit);
        command.Parameters.AddWithValue("$offset", filter.Offset);

        var rows = new List<BookmarkRow>();
        await using (var reader = await command.ExecuteReaderAsync(cancellationToken))
        {
            while (await reader.ReadAsync(cancellationToken))
            {
                rows.Add(ReadRow(reader));
            }
        }

        await HydrateTagsAsync(connection, null, rows, cancellationToken);
        return new BookmarkListResult(
            rows.Select(static row => row.ToDomain()).ToArray(),
            total,
            filter.Limit,
            filter.Offset);
    }

    public async Task<BookmarkItem> GetAsync(long id, CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await LoadRequiredAsync(connection, null, id, cancellationToken);
    }

    public Task<BookmarkItem> CreateAsync(
        CreateBookmarkRequest request,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    await EnsureUrlAvailableAsync(
                        connection, transaction, request.NormalizedUrl, null, token);
                    long id;
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            INSERT INTO bookmarks (url, url_normalized, title, description)
                            VALUES ($url, $normalizedUrl, $title, $description)
                            RETURNING id;
                            """;
                        command.Parameters.AddWithValue("$url", request.Url);
                        command.Parameters.AddWithValue("$normalizedUrl", request.NormalizedUrl);
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$description", request.Description);
                        id = Convert.ToInt64(await command.ExecuteScalarAsync(token));
                    }

                    await ReplaceTagsAsync(connection, transaction, id, request.Tags, token);
                    var bookmark = await LoadRequiredAsync(connection, transaction, id, token);
                    await transaction.CommitAsync(token);
                    return bookmark;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public Task<BookmarkItem> UpdateAsync(
        UpdateBookmarkRequest request,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    await EnsureUrlAvailableAsync(
                        connection, transaction, request.NormalizedUrl, request.Id, token);
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            UPDATE bookmarks
                            SET url = $url,
                                url_normalized = $normalizedUrl,
                                title = $title,
                                description = $description,
                                revision = revision + 1,
                                updated_at = CURRENT_TIMESTAMP
                            WHERE id = $id AND revision = $expectedRevision;
                            """;
                        command.Parameters.AddWithValue("$url", request.Url);
                        command.Parameters.AddWithValue("$normalizedUrl", request.NormalizedUrl);
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$description", request.Description);
                        command.Parameters.AddWithValue("$id", request.Id);
                        command.Parameters.AddWithValue("$expectedRevision", request.ExpectedRevision);
                        await RequireRevisionMutationAsync(
                            connection, transaction, command, request.Id, token);
                    }

                    await ReplaceTagsAsync(connection, transaction, request.Id, request.Tags, token);
                    var bookmark = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    await transaction.CommitAsync(token);
                    return bookmark;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public Task<BookmarkItem> SetReadAsync(
        SetBookmarkReadRequest request,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    var current = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    if (current.IsRead == request.IsRead)
                    {
                        await transaction.CommitAsync(token);
                        return current;
                    }

                    if (current.Revision != request.ExpectedRevision)
                    {
                        throw CreateConflict();
                    }

                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            UPDATE bookmarks
                            SET is_read = $isRead,
                                read_at = CASE WHEN $isRead = 1 THEN CURRENT_TIMESTAMP ELSE NULL END,
                                revision = revision + 1,
                                updated_at = CURRENT_TIMESTAMP
                            WHERE id = $id AND revision = $expectedRevision;
                            """;
                        command.Parameters.AddWithValue("$isRead", request.IsRead ? 1 : 0);
                        command.Parameters.AddWithValue("$id", request.Id);
                        command.Parameters.AddWithValue("$expectedRevision", request.ExpectedRevision);
                        await RequireRevisionMutationAsync(
                            connection, transaction, command, request.Id, token);
                    }

                    var bookmark = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    await transaction.CommitAsync(token);
                    return bookmark;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public Task DeleteAsync(long id, CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = "DELETE FROM bookmarks WHERE id = $id;";
                        command.Parameters.AddWithValue("$id", id);
                        if (await command.ExecuteNonQueryAsync(token) != 1)
                        {
                            throw new NotFoundException($"Bookmark {id} was not found.");
                        }
                    }

                    await RemoveUnusedTagsAsync(connection, transaction, token);
                    await transaction.CommitAsync(token);
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public async Task<IReadOnlyList<string>> ListTagsAsync(
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT name FROM bookmark_tags ORDER BY name_normalized ASC;";
        var tags = new List<string>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            tags.Add(reader.GetString(0));
        }

        return tags;
    }

    private static async Task<BookmarkItem> LoadRequiredAsync(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        long id,
        CancellationToken cancellationToken)
    {
        BookmarkRow? row;
        await using (var command = connection.CreateCommand())
        {
            command.Transaction = transaction;
            command.CommandText = """
                SELECT id, url, title, description, is_read, read_at,
                       revision, created_at, updated_at
                FROM bookmarks
                WHERE id = $id;
                """;
            command.Parameters.AddWithValue("$id", id);
            await using var reader = await command.ExecuteReaderAsync(cancellationToken);
            if (!await reader.ReadAsync(cancellationToken))
            {
                throw new NotFoundException($"Bookmark {id} was not found.");
            }

            row = ReadRow(reader);
        }

        await HydrateTagsAsync(connection, transaction, [row], cancellationToken);
        return row.ToDomain();
    }

    private static async Task HydrateTagsAsync(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        IReadOnlyList<BookmarkRow> rows,
        CancellationToken cancellationToken)
    {
        if (rows.Count == 0)
        {
            return;
        }

        var positions = rows.ToDictionary(static row => row.Id);
        var placeholders = new string[rows.Count];
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        for (var index = 0; index < rows.Count; index++)
        {
            placeholders[index] = $"$id{index}";
            command.Parameters.AddWithValue(placeholders[index], rows[index].Id);
        }

        command.CommandText = $"""
            SELECT bta.bookmark_id, bt.name
            FROM bookmark_tag_assignments AS bta
            JOIN bookmark_tags AS bt ON bt.id = bta.tag_id
            WHERE bta.bookmark_id IN ({string.Join(",", placeholders)})
            ORDER BY bt.name_normalized ASC;
            """;
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            if (positions.TryGetValue(reader.GetInt64(0), out var row))
            {
                row.Tags.Add(reader.GetString(1));
            }
        }
    }

    private static async Task ReplaceTagsAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        long bookmarkId,
        IReadOnlyList<string> tags,
        CancellationToken cancellationToken)
    {
        await ExecuteAsync(
            connection,
            transaction,
            "DELETE FROM bookmark_tag_assignments WHERE bookmark_id = $bookmarkId;",
            cancellationToken,
            ("$bookmarkId", bookmarkId));

        foreach (var tag in tags)
        {
            var normalized = tag.ToLowerInvariant();
            await ExecuteAsync(
                connection,
                transaction,
                """
                INSERT INTO bookmark_tags (name, name_normalized)
                VALUES ($name, $normalized)
                ON CONFLICT(name_normalized) DO NOTHING;
                """,
                cancellationToken,
                ("$name", tag),
                ("$normalized", normalized));

            long tagId;
            await using (var command = connection.CreateCommand())
            {
                command.Transaction = transaction;
                command.CommandText = "SELECT id FROM bookmark_tags WHERE name_normalized = $normalized;";
                command.Parameters.AddWithValue("$normalized", normalized);
                tagId = Convert.ToInt64(await command.ExecuteScalarAsync(cancellationToken));
            }

            await ExecuteAsync(
                connection,
                transaction,
                """
                INSERT INTO bookmark_tag_assignments (bookmark_id, tag_id)
                VALUES ($bookmarkId, $tagId);
                """,
                cancellationToken,
                ("$bookmarkId", bookmarkId),
                ("$tagId", tagId));
        }

        await RemoveUnusedTagsAsync(connection, transaction, cancellationToken);
    }

    private static Task RemoveUnusedTagsAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken)
    {
        return ExecuteAsync(
            connection,
            transaction,
            """
            DELETE FROM bookmark_tags
            WHERE NOT EXISTS (
                SELECT 1
                FROM bookmark_tag_assignments
                WHERE bookmark_tag_assignments.tag_id = bookmark_tags.id
            );
            """,
            cancellationToken);
    }

    private static async Task EnsureUrlAvailableAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string normalizedUrl,
        long? excludedId,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = excludedId is null
            ? "SELECT id FROM bookmarks WHERE url_normalized = $url LIMIT 1;"
            : "SELECT id FROM bookmarks WHERE url_normalized = $url AND id <> $excludedId LIMIT 1;";
        command.Parameters.AddWithValue("$url", normalizedUrl);
        if (excludedId is not null)
        {
            command.Parameters.AddWithValue("$excludedId", excludedId.Value);
        }

        if (await command.ExecuteScalarAsync(cancellationToken) is not null)
        {
            throw new ConflictException("This URL is already bookmarked.");
        }
    }

    private static async Task RequireRevisionMutationAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        SqliteCommand command,
        long id,
        CancellationToken cancellationToken)
    {
        var affected = await command.ExecuteNonQueryAsync(cancellationToken);
        if (affected == 1)
        {
            return;
        }

        if (affected != 0)
        {
            throw new InvalidOperationException($"Bookmark mutation affected {affected} rows.");
        }

        await using var existsCommand = connection.CreateCommand();
        existsCommand.Transaction = transaction;
        existsCommand.CommandText = "SELECT COUNT(*) FROM bookmarks WHERE id = $id;";
        existsCommand.Parameters.AddWithValue("$id", id);
        var exists = Convert.ToInt32(await existsCommand.ExecuteScalarAsync(cancellationToken)) == 1;
        if (!exists)
        {
            throw new NotFoundException($"Bookmark {id} was not found.");
        }

        throw CreateConflict();
    }

    private static ConflictException CreateConflict()
    {
        return new ConflictException("The bookmark changed after it was loaded.");
    }

    private static async Task ExecuteAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        string sql,
        CancellationToken cancellationToken,
        params (string Name, object Value)[] parameters)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = sql;
        foreach (var (name, value) in parameters)
        {
            command.Parameters.AddWithValue(name, value);
        }

        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static BookmarkRow ReadRow(SqliteDataReader reader)
    {
        return new BookmarkRow
        {
            Id = reader.GetInt64(0),
            Url = reader.GetString(1),
            Title = reader.GetString(2),
            Description = reader.GetString(3),
            IsRead = reader.GetInt64(4) != 0,
            ReadAt = reader.IsDBNull(5) ? null : ParseTimestamp(reader.GetString(5)),
            Revision = reader.GetInt32(6),
            CreatedAt = ParseTimestamp(reader.GetString(7)),
            UpdatedAt = ParseTimestamp(reader.GetString(8)),
        };
    }

    private static void CopyParameters(SqliteCommand source, SqliteCommand destination)
    {
        foreach (SqliteParameter parameter in source.Parameters)
        {
            destination.Parameters.AddWithValue(parameter.ParameterName, parameter.Value);
        }
    }

    private static string ToLikePattern(string query)
    {
        return "%" + query
            .ToLowerInvariant()
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("%", "\\%", StringComparison.Ordinal)
            .Replace("_", "\\_", StringComparison.Ordinal) + "%";
    }

    private static DateTimeOffset ParseTimestamp(string value)
    {
        if (DateTimeOffset.TryParseExact(
                value,
                "yyyy-MM-dd HH:mm:ss",
                CultureInfo.InvariantCulture,
                DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal,
                out var timestamp))
        {
            return timestamp;
        }

        return DateTimeOffset.TryParse(
            value,
            CultureInfo.InvariantCulture,
            DateTimeStyles.AssumeUniversal,
            out timestamp)
            ? timestamp
            : DateTimeOffset.MinValue;
    }

    private static async Task RollbackWithoutMaskingAsync(SqliteTransaction transaction)
    {
        try
        {
            await transaction.RollbackAsync(CancellationToken.None);
        }
        catch
        {
            // Preserve the original mutation failure.
        }
    }

    private sealed class BookmarkRow
    {
        public long Id { get; init; }
        public required string Url { get; init; }
        public required string Title { get; init; }
        public required string Description { get; init; }
        public bool IsRead { get; init; }
        public DateTimeOffset? ReadAt { get; init; }
        public int Revision { get; init; }
        public DateTimeOffset CreatedAt { get; init; }
        public DateTimeOffset UpdatedAt { get; init; }
        public List<string> Tags { get; } = [];

        public BookmarkItem ToDomain()
        {
            return new BookmarkItem(
                Id,
                Url,
                Title,
                Description,
                IsRead,
                ReadAt,
                Tags.ToArray(),
                Revision,
                CreatedAt,
                UpdatedAt);
        }
    }
}
