using System.Globalization;
using System.Text;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Application.Features.Tasks;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Tasks;

namespace Something.Infrastructure.Database.Repositories;

public sealed class TaskRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : ITaskRepository
{
    public async Task<IReadOnlyList<TaskItem>> SearchAsync(
        TaskSearchCriteria criteria,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await QueryAsync(
            connection,
            criteria,
            incompleteOnly: false,
            rangeStart: null,
            rangeEnd: null,
            cancellationToken);
    }

    public async Task<IReadOnlyList<TaskItem>> GetTodayIncompleteAsync(
        DateOnly today,
        CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await QueryAsync(
            connection,
            TaskSearchCriteria.Empty with { DueDate = today },
            incompleteOnly: true,
            rangeStart: null,
            rangeEnd: null,
            cancellationToken);
    }

    public async Task<IReadOnlyList<TaskItem>> GetRemainingWeekIncompleteAsync(
        DateOnly today,
        CancellationToken cancellationToken = default)
    {
        var mondayBasedDay = ((int)today.DayOfWeek + 6) % 7;
        var daysUntilSunday = 6 - mondayBasedDay;
        if (daysUntilSunday == 0)
        {
            return [];
        }

        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        return await QueryAsync(
            connection,
            TaskSearchCriteria.Empty,
            incompleteOnly: true,
            rangeStart: today.AddDays(1),
            rangeEnd: today.AddDays(daysUntilSunday),
            cancellationToken);
    }

    public Task<TaskItem> CreateAsync(
        CreateTaskRequest request,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);

                long taskId;
                try
                {
                    await using (var command = connection.CreateCommand())
                    {
                        command.Transaction = transaction;
                        command.CommandText = """
                            INSERT INTO todos (
                                title, description, is_completed, priority, due_date, difficulty
                            )
                            VALUES ($title, $description, 0, $priority, $dueDate, $difficulty)
                            RETURNING id;
                            """;
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$description", request.Description);
                        command.Parameters.AddWithValue("$priority", ToDatabase(request.Priority));
                        command.Parameters.AddWithValue(
                            "$dueDate",
                            request.DueDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ??
                            (object)DBNull.Value);
                        command.Parameters.AddWithValue(
                            "$difficulty",
                            request.Difficulty is null
                                ? DBNull.Value
                                : ToDatabase(request.Difficulty.Value));

                        taskId = Convert.ToInt64(await command.ExecuteScalarAsync(token));
                    }

                    await ReplaceTagsAsync(
                        connection,
                        transaction,
                        taskId,
                        request.Tags,
                        token);
                    await ReplaceSubtasksAsync(
                        connection,
                        transaction,
                        taskId,
                        request.Subtasks,
                        token);
                    await transaction.CommitAsync(token);
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }

                return await GetRequiredByIdAsync(connection, taskId, token);
            },
            cancellationToken);
    }

    public Task<TaskItem> UpdateAsync(
        UpdateTaskRequest request,
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
                            UPDATE todos
                            SET title = $title,
                                description = $description,
                                priority = $priority,
                                due_date = $dueDate,
                                difficulty = $difficulty
                            WHERE id = $id;
                            """;
                        command.Parameters.AddWithValue("$title", request.Title);
                        command.Parameters.AddWithValue("$description", request.Description);
                        command.Parameters.AddWithValue("$priority", ToDatabase(request.Priority));
                        command.Parameters.AddWithValue(
                            "$dueDate",
                            request.DueDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ??
                            (object)DBNull.Value);
                        command.Parameters.AddWithValue(
                            "$difficulty",
                            request.Difficulty is null
                                ? DBNull.Value
                                : ToDatabase(request.Difficulty.Value));
                        command.Parameters.AddWithValue("$id", request.Id);

                        if (await command.ExecuteNonQueryAsync(token) != 1)
                        {
                            throw new NotFoundException($"Task {request.Id} was not found.");
                        }
                    }

                    await ReplaceTagsAsync(
                        connection,
                        transaction,
                        request.Id,
                        request.Tags,
                        token);
                    await ReplaceSubtasksAsync(
                        connection,
                        transaction,
                        request.Id,
                        request.Subtasks,
                        token);
                    await transaction.CommitAsync(token);
                }
                catch
                {
                    await RollbackWithoutMaskingAsync(transaction);
                    throw;
                }

                return await GetRequiredByIdAsync(connection, request.Id, token);
            },
            cancellationToken);
    }

    public Task ToggleAsync(long id, CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    UPDATE todos
                    SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END
                    WHERE id = $id;
                    """;
                command.Parameters.AddWithValue("$id", id);

                if (await command.ExecuteNonQueryAsync(token) != 1)
                {
                    throw new NotFoundException($"Task {id} was not found.");
                }
            },
            cancellationToken);
    }

    public Task ToggleSubtaskAsync(
        long taskId,
        long subtaskId,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    UPDATE todo_subtasks
                    SET is_completed = CASE WHEN is_completed = 0 THEN 1 ELSE 0 END
                    WHERE id = $subtaskId
                      AND todo_id = $taskId
                      AND EXISTS (
                          SELECT 1
                          FROM todos
                          WHERE todos.id = todo_subtasks.todo_id
                            AND todos.difficulty = 'hard'
                      );
                    """;
                command.Parameters.AddWithValue("$subtaskId", subtaskId);
                command.Parameters.AddWithValue("$taskId", taskId);

                if (await command.ExecuteNonQueryAsync(token) != 1)
                {
                    throw new NotFoundException($"Subtask {subtaskId} was not found.");
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
                        command.CommandText = "DELETE FROM todos WHERE id = $id;";
                        command.Parameters.AddWithValue("$id", id);
                        if (await command.ExecuteNonQueryAsync(token) != 1)
                        {
                            throw new NotFoundException($"Task {id} was not found.");
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

    private static async Task<IReadOnlyList<TaskItem>> QueryAsync(
        SqliteConnection connection,
        TaskSearchCriteria criteria,
        bool incompleteOnly,
        DateOnly? rangeStart,
        DateOnly? rangeEnd,
        CancellationToken cancellationToken)
    {
        var sql = new StringBuilder("""
            SELECT t.id,
                   t.title,
                   COALESCE(t.description, ''),
                   t.is_completed,
                   t.created_at,
                   t.due_date,
                   t.priority,
                   t.difficulty
            FROM todos AS t
            WHERE 1 = 1
            """);
        sql.AppendLine();
        await using var command = connection.CreateCommand();

        if (incompleteOnly)
        {
            sql.AppendLine(" AND t.is_completed = 0");
        }

        if (!string.IsNullOrEmpty(criteria.Query))
        {
            sql.AppendLine("""
                AND (
                    LOWER(COALESCE(t.title, '')) LIKE $query ESCAPE '\'
                    OR LOWER(COALESCE(t.description, '')) LIKE $query ESCAPE '\'
                    OR EXISTS (
                        SELECT 1
                        FROM todo_tags AS search_todo_tags
                        JOIN tags AS search_tags ON search_tags.id = search_todo_tags.tag_id
                        WHERE search_todo_tags.todo_id = t.id
                          AND search_tags.name_normalized LIKE $query ESCAPE '\'
                    )
                    OR EXISTS (
                        SELECT 1
                        FROM todo_subtasks AS search_subtasks
                        WHERE search_subtasks.todo_id = t.id
                          AND LOWER(search_subtasks.title) LIKE $query ESCAPE '\'
                    )
                )
                """);
            command.Parameters.AddWithValue("$query", ToLikePattern(criteria.Query));
        }

        if (criteria.DueDate is not null)
        {
            sql.AppendLine(" AND t.due_date = $dueDate");
            command.Parameters.AddWithValue(
                "$dueDate",
                criteria.DueDate.Value.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture));
        }

        if (criteria.Priority is not null)
        {
            sql.AppendLine(" AND t.priority = $priority");
            command.Parameters.AddWithValue("$priority", ToDatabase(criteria.Priority.Value));
        }

        if (criteria.DifficultyIsUnset)
        {
            sql.AppendLine(" AND (t.difficulty IS NULL OR t.difficulty = '')");
        }
        else if (criteria.Difficulty is not null)
        {
            sql.AppendLine(" AND t.difficulty = $difficulty");
            command.Parameters.AddWithValue("$difficulty", ToDatabase(criteria.Difficulty.Value));
        }

        for (var index = 0; index < criteria.Tags.Count; index++)
        {
            var parameterName = $"$tag{index}";
            sql.AppendLine($$"""
                AND EXISTS (
                    SELECT 1
                    FROM todo_tags AS filter_todo_tags
                    JOIN tags AS filter_tags ON filter_tags.id = filter_todo_tags.tag_id
                    WHERE filter_todo_tags.todo_id = t.id
                      AND filter_tags.name_normalized = {{parameterName}}
                )
                """);
            command.Parameters.AddWithValue(parameterName, criteria.Tags[index].ToLowerInvariant());
        }

        if (rangeStart is not null && rangeEnd is not null)
        {
            sql.AppendLine(" AND t.due_date BETWEEN $rangeStart AND $rangeEnd");
            command.Parameters.AddWithValue(
                "$rangeStart",
                rangeStart.Value.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture));
            command.Parameters.AddWithValue(
                "$rangeEnd",
                rangeEnd.Value.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture));
        }

        if (rangeStart is not null)
        {
            sql.AppendLine(" ORDER BY t.due_date ASC,");
            AppendPriorityOrder(sql);
            sql.AppendLine(", t.created_at ASC");
        }
        else if (criteria.DueDate is not null)
        {
            sql.AppendLine(" ORDER BY");
            AppendPriorityOrder(sql);
            sql.AppendLine(", t.created_at ASC");
        }
        else
        {
            sql.AppendLine(" ORDER BY t.created_at DESC");
        }

        command.CommandText = sql.ToString();
        var rows = new List<TaskRow>();

        await using (var reader = await command.ExecuteReaderAsync(cancellationToken))
        {
            while (await reader.ReadAsync(cancellationToken))
            {
                rows.Add(ReadTaskRow(reader));
            }
        }

        await HydrateRelationsAsync(connection, rows, cancellationToken);
        return rows.Select(row => row.ToDomain()).ToArray();
    }

    private static async Task<TaskItem> GetRequiredByIdAsync(
        SqliteConnection connection,
        long id,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.CommandText = """
            SELECT t.id,
                   t.title,
                   COALESCE(t.description, ''),
                   t.is_completed,
                   t.created_at,
                   t.due_date,
                   t.priority,
                   t.difficulty
            FROM todos AS t
            WHERE t.id = $id;
            """;
        command.Parameters.AddWithValue("$id", id);

        TaskRow? row = null;
        await using (var reader = await command.ExecuteReaderAsync(cancellationToken))
        {
            if (await reader.ReadAsync(cancellationToken))
            {
                row = ReadTaskRow(reader);
            }
        }

        if (row is null)
        {
            throw new NotFoundException($"Task {id} was not found.");
        }

        var rows = new List<TaskRow> { row };
        await HydrateRelationsAsync(connection, rows, cancellationToken);
        return row.ToDomain();
    }

    private static TaskRow ReadTaskRow(SqliteDataReader reader)
    {
        return new TaskRow
        {
            Id = reader.GetInt64(0),
            Title = reader.GetString(1),
            Description = reader.GetString(2),
            IsCompleted = reader.GetInt64(3) != 0,
            CreatedAt = ParseCreatedAt(reader.GetString(4)),
            DueDate = reader.IsDBNull(5)
                ? null
                : DateOnly.ParseExact(
                    reader.GetString(5),
                    "yyyy-MM-dd",
                    CultureInfo.InvariantCulture),
            Priority = ParsePriority(reader.GetString(6)),
            Difficulty = reader.IsDBNull(7) ? null : ParseDifficulty(reader.GetString(7)),
        };
    }

    private static async Task HydrateRelationsAsync(
        SqliteConnection connection,
        IReadOnlyCollection<TaskRow> rows,
        CancellationToken cancellationToken)
    {
        if (rows.Count == 0)
        {
            return;
        }

        var byId = rows.ToDictionary(row => row.Id);

        await using (var tagCommand = connection.CreateCommand())
        {
            tagCommand.CommandText = """
                SELECT todo_tags.todo_id, tags.name
                FROM todo_tags
                JOIN tags ON tags.id = todo_tags.tag_id
                ORDER BY todo_tags.todo_id, LOWER(tags.name), tags.id;
                """;
            await using var reader = await tagCommand.ExecuteReaderAsync(cancellationToken);
            while (await reader.ReadAsync(cancellationToken))
            {
                if (byId.TryGetValue(reader.GetInt64(0), out var row))
                {
                    row.Tags.Add(reader.GetString(1));
                }
            }
        }

        await using var subtaskCommand = connection.CreateCommand();
        subtaskCommand.CommandText = """
            SELECT id, todo_id, title, is_completed, position
            FROM todo_subtasks
            ORDER BY todo_id, position, id;
            """;
        await using var subtaskReader = await subtaskCommand.ExecuteReaderAsync(cancellationToken);
        while (await subtaskReader.ReadAsync(cancellationToken))
        {
            if (byId.TryGetValue(subtaskReader.GetInt64(1), out var row))
            {
                row.Subtasks.Add(new TaskSubtask(
                    subtaskReader.GetInt64(0),
                    subtaskReader.GetString(2),
                    subtaskReader.GetInt64(3) != 0,
                    subtaskReader.GetInt32(4)));
            }
        }
    }

    private static async Task ReplaceTagsAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        long taskId,
        IReadOnlyList<string> tags,
        CancellationToken cancellationToken)
    {
        await ExecuteAsync(
            connection,
            transaction,
            "DELETE FROM todo_tags WHERE todo_id = $taskId;",
            cancellationToken,
            ("$taskId", taskId));

        foreach (var name in tags)
        {
            var normalized = name.ToLowerInvariant();
            await ExecuteAsync(
                connection,
                transaction,
                """
                INSERT INTO tags (name, name_normalized)
                VALUES ($name, $normalized)
                ON CONFLICT(name_normalized) DO NOTHING;
                """,
                cancellationToken,
                ("$name", name),
                ("$normalized", normalized));

            await ExecuteAsync(
                connection,
                transaction,
                """
                INSERT INTO todo_tags (todo_id, tag_id)
                SELECT $taskId, id
                FROM tags
                WHERE name_normalized = $normalized;
                """,
                cancellationToken,
                ("$taskId", taskId),
                ("$normalized", normalized));
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
            DELETE FROM tags
            WHERE NOT EXISTS (
                SELECT 1
                FROM todo_tags
                WHERE todo_tags.tag_id = tags.id
            );
            """,
            cancellationToken);
    }

    private static async Task ReplaceSubtasksAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        long taskId,
        IReadOnlyList<TaskSubtaskInput> subtasks,
        CancellationToken cancellationToken)
    {
        var existingIds = new HashSet<long>();
        await using (var query = connection.CreateCommand())
        {
            query.Transaction = transaction;
            query.CommandText = "SELECT id FROM todo_subtasks WHERE todo_id = $taskId;";
            query.Parameters.AddWithValue("$taskId", taskId);
            await using var reader = await query.ExecuteReaderAsync(cancellationToken);
            while (await reader.ReadAsync(cancellationToken))
            {
                existingIds.Add(reader.GetInt64(0));
            }
        }

        var retainedIds = new HashSet<long>();
        foreach (var subtask in subtasks)
        {
            if (subtask.Id == 0)
            {
                await ExecuteAsync(
                    connection,
                    transaction,
                    """
                    INSERT INTO todo_subtasks (todo_id, title, is_completed, position)
                    VALUES ($taskId, $title, $isCompleted, $position);
                    """,
                    cancellationToken,
                    ("$taskId", taskId),
                    ("$title", subtask.Title),
                    ("$isCompleted", subtask.IsCompleted ? 1 : 0),
                    ("$position", subtask.Position));
                continue;
            }

            if (!existingIds.Contains(subtask.Id))
            {
                throw new ValidationException(
                    $"Subtask {subtask.Id} does not belong to task {taskId}.");
            }

            await ExecuteAsync(
                connection,
                transaction,
                """
                UPDATE todo_subtasks
                SET title = $title,
                    is_completed = $isCompleted,
                    position = $position
                WHERE id = $subtaskId AND todo_id = $taskId;
                """,
                cancellationToken,
                ("$title", subtask.Title),
                ("$isCompleted", subtask.IsCompleted ? 1 : 0),
                ("$position", subtask.Position),
                ("$subtaskId", subtask.Id),
                ("$taskId", taskId));
            retainedIds.Add(subtask.Id);
        }

        foreach (var existingId in existingIds)
        {
            if (retainedIds.Contains(existingId))
            {
                continue;
            }

            await ExecuteAsync(
                connection,
                transaction,
                """
                DELETE FROM todo_subtasks
                WHERE id = $subtaskId AND todo_id = $taskId;
                """,
                cancellationToken,
                ("$subtaskId", existingId),
                ("$taskId", taskId));
        }
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
        foreach (var parameter in parameters)
        {
            command.Parameters.AddWithValue(parameter.Name, parameter.Value);
        }

        await command.ExecuteNonQueryAsync(cancellationToken);
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

    private static void AppendPriorityOrder(StringBuilder sql)
    {
        sql.Append("""
            CASE t.priority
                WHEN 'high' THEN 0
                WHEN 'medium' THEN 1
                WHEN 'low' THEN 2
                ELSE 3
            END
            """);
    }

    private static string ToLikePattern(string query)
    {
        return "%" + query
            .ToLowerInvariant()
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("%", "\\%", StringComparison.Ordinal)
            .Replace("_", "\\_", StringComparison.Ordinal) + "%";
    }

    private static string ToDatabase(TaskPriority priority)
    {
        return priority.ToString().ToLowerInvariant();
    }

    private static string ToDatabase(TaskDifficulty difficulty)
    {
        return difficulty.ToString().ToLowerInvariant();
    }

    private static TaskPriority ParsePriority(string value)
    {
        return Enum.TryParse<TaskPriority>(value, true, out var priority)
            ? priority
            : TaskPriority.Medium;
    }

    private static TaskDifficulty? ParseDifficulty(string value)
    {
        return Enum.TryParse<TaskDifficulty>(value, true, out var difficulty)
            ? difficulty
            : null;
    }

    private static DateTimeOffset ParseCreatedAt(string value)
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

    private sealed class TaskRow
    {
        public long Id { get; init; }
        public required string Title { get; init; }
        public required string Description { get; init; }
        public bool IsCompleted { get; init; }
        public DateTimeOffset CreatedAt { get; init; }
        public DateOnly? DueDate { get; init; }
        public TaskPriority Priority { get; init; }
        public TaskDifficulty? Difficulty { get; init; }
        public List<string> Tags { get; } = [];
        public List<TaskSubtask> Subtasks { get; } = [];

        public TaskItem ToDomain()
        {
            return new TaskItem(
                Id,
                Title,
                Description,
                IsCompleted,
                CreatedAt,
                DueDate,
                Priority,
                Difficulty,
                Tags.ToArray(),
                Subtasks.ToArray());
        }
    }
}
