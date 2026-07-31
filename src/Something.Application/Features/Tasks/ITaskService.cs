using Something.Domain.Models.Tasks;

namespace Something.Application.Features.Tasks;

public interface ITaskService
{
    Task<IReadOnlyList<TaskItem>> SearchAsync(
        TaskSearchCriteria? criteria = null,
        CancellationToken cancellationToken = default);

    Task<IReadOnlyList<TaskItem>> GetTodayIncompleteAsync(
        CancellationToken cancellationToken = default);

    Task<IReadOnlyList<TaskItem>> GetRemainingWeekIncompleteAsync(
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
