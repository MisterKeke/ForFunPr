using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Tasks;

namespace Something.Domain.Validation;

public static class TaskRules
{
    public const int MaximumSearchLength = 256;
    public const int MaximumTagLength = 64;
    public const int MaximumTagCount = 32;
    public const int MaximumSubtaskCount = 100;

    public static string NormalizeTitle(string? title)
    {
        var normalized = title?.Trim() ?? string.Empty;
        if (normalized.Length == 0)
        {
            throw new ValidationException("Task title cannot be empty.");
        }

        return normalized;
    }

    public static string NormalizeDescription(string? description)
    {
        return description?.Trim() ?? string.Empty;
    }

    public static string NormalizeSearch(string? query)
    {
        var normalized = query?.Trim() ?? string.Empty;
        if (normalized.Length > MaximumSearchLength)
        {
            throw new ValidationException(
                $"Task search cannot exceed {MaximumSearchLength} characters.");
        }

        return normalized;
    }

    public static IReadOnlyList<string> NormalizeTags(IEnumerable<string>? values)
    {
        var tags = new List<string>();
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

        foreach (var value in values ?? [])
        {
            var name = value.Trim();
            if (name.Length == 0)
            {
                throw new ValidationException("Tag names cannot be empty.");
            }

            if (name.Length > MaximumTagLength)
            {
                throw new ValidationException(
                    $"Tag names cannot exceed {MaximumTagLength} characters.");
            }

            if (seen.Add(name))
            {
                tags.Add(name);
            }
        }

        if (tags.Count > MaximumTagCount)
        {
            throw new ValidationException(
                $"A task cannot have more than {MaximumTagCount} tags.");
        }

        return tags;
    }

    public static IReadOnlyList<TaskSubtaskInput> NormalizeSubtasks(
        IEnumerable<TaskSubtaskInput>? values,
        TaskDifficulty? difficulty)
    {
        var source = (values ?? []).ToArray();
        if (source.Length > 0 && difficulty != TaskDifficulty.Hard)
        {
            throw new ValidationException("Subtasks are only allowed for hard tasks.");
        }

        if (source.Length > MaximumSubtaskCount)
        {
            throw new ValidationException(
                $"A task cannot have more than {MaximumSubtaskCount} subtasks.");
        }

        var normalized = new List<TaskSubtaskInput>(source.Length);
        var seenIds = new HashSet<long>();

        for (var index = 0; index < source.Length; index++)
        {
            var subtask = source[index];
            if (subtask.Id < 0)
            {
                throw new ValidationException("Subtask IDs cannot be negative.");
            }

            if (subtask.Id > 0 && !seenIds.Add(subtask.Id))
            {
                throw new ValidationException("Subtask IDs cannot be repeated.");
            }

            var title = subtask.Title.Trim();
            if (title.Length == 0)
            {
                throw new ValidationException("Subtask titles cannot be empty.");
            }

            normalized.Add(subtask with { Title = title, Position = index });
        }

        return normalized;
    }

    public static long RequirePositiveId(long id, string resourceName)
    {
        if (id <= 0)
        {
            throw new ValidationException($"{resourceName} ID must be a positive integer.");
        }

        return id;
    }
}
