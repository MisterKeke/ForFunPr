using Something.Application.Features.Tasks;
using Something.Domain.Models.Tasks;

namespace Something.Application.Abstractions.Persistence;

public interface ITaskRepository
{
    Task<IReadOnlyList<TaskItem>> SearchAsync(
        TaskSearchCriteria criteria,
        CancellationToken cancellationToken = default);

    Task<IReadOnlyList<TaskItem>> GetTodayIncompleteAsync(
        DateOnly today,
        CancellationToken cancellationToken = default);

    Task<IReadOnlyList<TaskItem>> GetRemainingWeekIncompleteAsync(
        DateOnly today,
        CancellationToken cancellationToken = default);

    Task<TaskItem> CreateAsync(
        CreateTaskRequest request,
        CancellationToken cancellationToken = default);

    Task<TaskItem> UpdateAsync(
        UpdateTaskRequest request,
        CancellationToken cancellationToken = default);

    Task ToggleAsync(long id, CancellationToken cancellationToken = default);

    Task ToggleSubtaskAsync(
        long taskId,
        long subtaskId,
        CancellationToken cancellationToken = default);

    Task DeleteAsync(long id, CancellationToken cancellationToken = default);
}
