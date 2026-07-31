using Something.Domain.Enums;

namespace Something.Domain.Models.Tasks;

public sealed record TaskItem(
    long Id,
    string Title,
    string Description,
    bool IsCompleted,
    DateTimeOffset CreatedAt,
    DateOnly? DueDate,
    TaskPriority Priority,
    TaskDifficulty? Difficulty,
    IReadOnlyList<string> Tags,
    IReadOnlyList<TaskSubtask> Subtasks)
{
    public bool IsOverdue(DateOnly today)
    {
        return !IsCompleted && DueDate is not null && DueDate.Value < today;
    }
}
