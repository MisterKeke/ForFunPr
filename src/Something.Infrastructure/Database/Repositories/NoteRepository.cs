using System.Globalization;
using System.Text;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Application.Features.Notes;
using Something.Domain.Exceptions;
using Something.Domain.Models.Notes;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class NoteRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : INoteRepository
{
    public async Task<NoteListResult> ListAsync(
        NoteListFilter filter,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        var clauses = new List<string> { "1 = 1" };
        var parameters = new List<(string Name, object Value)>();

        switch (filter.ArchiveStatus)
        {
            case NoteArchiveStatus.Active:
                clauses.Add("is_archived = 0");
                break;
            case NoteArchiveStatus.Archived:
                clauses.Add("is_archived = 1");
                break;
            case NoteArchiveStatus.All:
                break;
            default:
                throw new ValidationException("Unknown note archive filter.");
        }

        if (filter.IsPinned is not null)
        {
            clauses.Add("is_pinned = $isPinned");
            parameters.Add(("$isPinned", filter.IsPinned.Value ? 1 : 0));
        }

        if (filter.Query.Length > 0)
        {
            clauses.Add("(LOWER(title) LIKE $query ESCAPE '\\' OR LOWER(body) LIKE $query ESCAPE '\\')");
            parameters.Add(("$query", ToLikePattern(filter.Query)));
        }

        var whereSql = string.Join(" AND ", clauses);
        int total;
        await using (var countCommand = connection.CreateCommand())
        {
            countCommand.CommandText = $"SELECT COUNT(*) FROM notes WHERE {whereSql};";
            AddParameters(countCommand, parameters);
            total = Convert.ToInt32(await countCommand.ExecuteScalarAsync(cancellationToken));
        }

        await using var command = connection.CreateCommand();
        command.CommandText = $"""
            SELECT id, title, substr(body, 1, 240), is_pinned, is_archived,
                   revision, created_at, updated_at
            FROM notes
            WHERE {whereSql}
            ORDER BY is_pinned DESC, updated_at DESC, id DESC
            LIMIT $limit OFFSET $offset;
            """;
        AddParameters(command, parameters);
        command.Parameters.AddWithValue("$limit", filter.Limit);
        command.Parameters.AddWithValue("$offset", filter.Offset);

        var notes = new List<NoteSummary>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            notes.Add(new NoteSummary(
                reader.GetInt64(0),
                reader.GetString(1),
                NoteRules.CreatePreview(reader.GetString(2)),
                reader.GetInt64(3) != 0,
                reader.GetInt64(4) != 0,
                reader.GetInt32(5),
                ParseTimestamp(reader.GetString(6)),
                ParseTimestamp(reader.GetString(7))));
        }

        return new NoteListResult(notes, total, filter.Limit, filter.Offset);
    }

    public async Task<NoteItem> GetAsync(long id, CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await LoadRequiredAsync(connection, null, id, cancellationToken);
    }

    public Task<NoteItem> CreateAsync(
        CreateNoteRequest request,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    long id;
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            INSERT INTO notes (title, body, is_pinned)
                            VALUES ($title, $body, $isPinned)
                            RETURNING id;
                            """;
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$body", request.Body);
                        command.Parameters.AddWithValue("$isPinned", request.IsPinned ? 1 : 0);
                        id = Convert.ToInt64(await command.ExecuteScalarAsync(token));
                    }

                    var note = await LoadRequiredAsync(connection, transaction, id, token);
                    await transaction.CommitAsync(token);
                    return note;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public Task<NoteItem> UpdateAsync(
        UpdateNoteRequest request,
        CancellationToken cancellationToken = default)
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
                        command.CommandText = """
                            UPDATE notes
                            SET title = $title,
                                body = $body,
                                revision = revision + 1,
                                updated_at = CURRENT_TIMESTAMP
                            WHERE id = $id AND revision = $expectedRevision;
                            """;
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$body", request.Body);
                        command.Parameters.AddWithValue("$id", request.Id);
                        command.Parameters.AddWithValue("$expectedRevision", request.ExpectedRevision);
                        await RequireRevisionMutationAsync(
                            connection,
                            transaction,
                            command,
                            request.Id,
                            token);
                    }

                    var note = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    await transaction.CommitAsync(token);
                    return note;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    public Task<NoteItem> SetPinnedAsync(
        SetNoteStateRequest request,
        CancellationToken cancellationToken = default)
    {
        return SetStateAsync(request, isPinned: true, cancellationToken);
    }

    public Task<NoteItem> SetArchivedAsync(
        SetNoteStateRequest request,
        CancellationToken cancellationToken = default)
    {
        return SetStateAsync(request, isPinned: false, cancellationToken);
    }

    public Task DeleteAsync(long id, CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = "DELETE FROM notes WHERE id = $id;";
                command.Parameters.AddWithValue("$id", id);
                if (await command.ExecuteNonQueryAsync(token) != 1)
                {
                    throw new NotFoundException($"Note {id} was not found.");
                }
            },
            cancellationToken);
    }

    private Task<NoteItem> SetStateAsync(
        SetNoteStateRequest request,
        bool isPinned,
        CancellationToken cancellationToken)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    var current = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    var currentValue = isPinned ? current.IsPinned : current.IsArchived;
                    if (currentValue == request.Value)
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
                        command.CommandText = isPinned
                            ? """
                                UPDATE notes
                                SET is_pinned = $value, revision = revision + 1,
                                    updated_at = CURRENT_TIMESTAMP
                                WHERE id = $id AND revision = $expectedRevision;
                                """
                            : """
                                UPDATE notes
                                SET is_archived = $value, revision = revision + 1,
                                    updated_at = CURRENT_TIMESTAMP
                                WHERE id = $id AND revision = $expectedRevision;
                                """;
                        command.Parameters.AddWithValue("$value", request.Value ? 1 : 0);
                        command.Parameters.AddWithValue("$id", request.Id);
                        command.Parameters.AddWithValue("$expectedRevision", request.ExpectedRevision);
                        await RequireRevisionMutationAsync(
                            connection,
                            transaction,
                            command,
                            request.Id,
                            token);
                    }

                    var note = await LoadRequiredAsync(connection, transaction, request.Id, token);
                    await transaction.CommitAsync(token);
                    return note;
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }
            },
            cancellationToken);
    }

    private static async Task<NoteItem> LoadRequiredAsync(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        long id,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = """
            SELECT id, title, body, is_pinned, is_archived, revision,
                   created_at, updated_at
            FROM notes
            WHERE id = $id;
            """;
        command.Parameters.AddWithValue("$id", id);

        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        if (!await reader.ReadAsync(cancellationToken))
        {
            throw new NotFoundException($"Note {id} was not found.");
        }

        return new NoteItem(
            reader.GetInt64(0),
            reader.GetString(1),
            reader.GetString(2),
            reader.GetInt64(3) != 0,
            reader.GetInt64(4) != 0,
            reader.GetInt32(5),
            ParseTimestamp(reader.GetString(6)),
            ParseTimestamp(reader.GetString(7)));
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
            throw new InvalidOperationException($"Note mutation affected {affected} rows.");
        }

        await using var existsCommand = connection.CreateCommand();
        existsCommand.Transaction = transaction;
        existsCommand.CommandText = "SELECT COUNT(*) FROM notes WHERE id = $id;";
        existsCommand.Parameters.AddWithValue("$id", id);
        var exists = Convert.ToInt32(await existsCommand.ExecuteScalarAsync(cancellationToken)) == 1;
        if (!exists)
        {
            throw new NotFoundException($"Note {id} was not found.");
        }

        throw CreateConflict();
    }

    private static ConflictException CreateConflict()
    {
        return new ConflictException("The note changed after it was loaded.");
    }

    private static void AddParameters(
        SqliteCommand command,
        IEnumerable<(string Name, object Value)> parameters)
    {
        foreach (var (name, value) in parameters)
        {
            command.Parameters.AddWithValue(name, value);
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
}
