using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Messages;
using Something.Domain.Models.Tasks;
using Something.Domain.Validation;

namespace Something.Application.Features.Tasks;

public sealed class TaskService(
    ITaskRepository repository,
    IMessenger messenger) : ITaskService
{
    public Task<IReadOnlyList<TaskItem>> SearchAsync(
        TaskSearchCriteria? criteria = null,
        CancellationToken cancellationToken = default)
    {
        criteria ??= TaskSearchCriteria.Empty;

        var normalized = criteria with
        {
            Query = TaskRules.NormalizeSearch(criteria.Query),
            Tags = TaskRules.NormalizeTags(criteria.Tags),
        };

        if (normalized.DifficultyIsUnset && normalized.Difficulty is not null)
        {
            throw new Domain.Exceptions.ValidationException(
                "Difficulty cannot be both unset and a specific value.");
        }

        return repository.SearchAsync(normalized, cancellationToken);
    }

    public Task<IReadOnlyList<TaskItem>> GetTodayIncompleteAsync(
        CancellationToken cancellationToken = default)
    {
        return repository.GetTodayIncompleteAsync(
            DateOnly.FromDateTime(DateTime.Now),
            cancellationToken);
    }

    public Task<IReadOnlyList<TaskItem>> GetRemainingWeekIncompleteAsync(
        CancellationToken cancellationToken = default)
    {
        return repository.GetRemainingWeekIncompleteAsync(
            DateOnly.FromDateTime(DateTime.Now),
            cancellationToken);
    }

    public async Task<TaskItem> CreateAsync(
        CreateTaskRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        var normalized = request with
        {
            Title = TaskRules.NormalizeTitle(request.Title),
            Description = TaskRules.NormalizeDescription(request.Description),
            Tags = TaskRules.NormalizeTags(request.Tags),
            Subtasks = TaskRules.NormalizeSubtasks(request.Subtasks, request.Difficulty),
        };

        var created = await repository.CreateAsync(normalized, cancellationToken);
        PublishChanged();
        return created;
    }

    public async Task<TaskItem> UpdateAsync(
        UpdateTaskRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        TaskRules.RequirePositiveId(request.Id, "Task");
        var normalized = request with
        {
            Title = TaskRules.NormalizeTitle(request.Title),
            Description = TaskRules.NormalizeDescription(request.Description),
            Tags = TaskRules.NormalizeTags(request.Tags),
            Subtasks = TaskRules.NormalizeSubtasks(request.Subtasks, request.Difficulty),
        };

        var updated = await repository.UpdateAsync(normalized, cancellationToken);
        PublishChanged();
        return updated;
    }

    public async Task ToggleAsync(long id, CancellationToken cancellationToken = default)
    {
        TaskRules.RequirePositiveId(id, "Task");
        await repository.ToggleAsync(id, cancellationToken);
        PublishChanged();
    }

    public async Task ToggleSubtaskAsync(
        long taskId,
        long subtaskId,
        CancellationToken cancellationToken = default)
    {
        TaskRules.RequirePositiveId(taskId, "Task");
        TaskRules.RequirePositiveId(subtaskId, "Subtask");
        await repository.ToggleSubtaskAsync(taskId, subtaskId, cancellationToken);
        PublishChanged();
    }

    public async Task DeleteAsync(long id, CancellationToken cancellationToken = default)
    {
        TaskRules.RequirePositiveId(id, "Task");
        await repository.DeleteAsync(id, cancellationToken);
        PublishChanged();
    }

    private void PublishChanged()
    {
        messenger.Send(new TasksChangedMessage(DateTimeOffset.UtcNow));
    }
}
