using Something.Domain.Enums;
using Something.Domain.Models.Tasks;

namespace Something.Application.Features.Tasks;

public sealed record CreateTaskRequest(
    string Title,
    string Description,
    TaskPriority Priority,
    DateOnly? DueDate,
    TaskDifficulty? Difficulty,
    IReadOnlyList<string> Tags,
    IReadOnlyList<TaskSubtaskInput> Subtasks);
