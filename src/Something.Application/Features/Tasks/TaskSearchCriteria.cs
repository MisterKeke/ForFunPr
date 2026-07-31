using Something.Domain.Enums;

namespace Something.Application.Features.Tasks;

public sealed record TaskSearchCriteria(
    string Query,
    DateOnly? DueDate,
    TaskPriority? Priority,
    TaskDifficulty? Difficulty,
    bool DifficultyIsUnset,
    IReadOnlyList<string> Tags)
{
    public static TaskSearchCriteria Empty { get; } = new(
        string.Empty,
        null,
        null,
        null,
        false,
        []);
}
