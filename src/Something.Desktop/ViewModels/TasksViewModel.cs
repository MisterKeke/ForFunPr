using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Tasks;
using Something.Application.Abstractions.Dialogs;
using Something.Desktop.Navigation;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Tasks;

namespace Something.Desktop.ViewModels;

public sealed partial class TasksViewModel(
    ITaskService taskService,
    IConfirmationDialog confirmationDialog,
    ILogger<TasksViewModel> logger) : PageViewModel
{
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.Tasks;
    public override string Title => "Tasks";
    public override string Subtitle => "Plan work, track due dates, and break down difficult tasks.";

    public ObservableCollection<TaskItem> Tasks { get; } = [];
    public ObservableCollection<TaskItem> CalendarTasks { get; } = [];
    public ObservableCollection<TaskSubtaskDraftViewModel> DraftSubtasks { get; } = [];

    public bool IsHardDifficulty => DraftDifficulty == TaskDifficulty.Hard;
    public string EditorHeading => EditingTaskId is null ? "Add new task" : "Edit task";

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private DateTime? _filterDueDate;

    [ObservableProperty]
    private string _selectedPriorityFilter = string.Empty;

    [ObservableProperty]
    private string _selectedDifficultyFilter = string.Empty;

    [ObservableProperty]
    private string _filterTagsText = string.Empty;

    [ObservableProperty]
    private DateTime? _selectedCalendarDate = DateTime.Today;

    [ObservableProperty]
    private bool _isCalendarBusy;

    [ObservableProperty]
    private bool _isEditorOpen;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(EditorHeading))]
    private long? _editingTaskId;

    [ObservableProperty]
    private string _draftTitle = string.Empty;

    [ObservableProperty]
    private string _draftDescription = string.Empty;

    [ObservableProperty]
    private DateTime? _draftDueDate;

    [ObservableProperty]
    private TaskPriority _draftPriority = TaskPriority.Medium;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsHardDifficulty))]
    private TaskDifficulty? _draftDifficulty;

    [ObservableProperty]
    private string _draftTagsText = string.Empty;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await LoadTasksAsync(_pageCancellation.Token);
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
        IsEditorOpen = false;
    }

    [RelayCommand]
    private Task ApplyFiltersAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(LoadTasksAsync, cancellationToken);
    }

    [RelayCommand]
    private async Task ClearFiltersAsync(CancellationToken cancellationToken)
    {
        SearchText = string.Empty;
        FilterDueDate = null;
        SelectedPriorityFilter = string.Empty;
        SelectedDifficultyFilter = string.Empty;
        FilterTagsText = string.Empty;
        await RunBusyAsync(LoadTasksAsync, cancellationToken);
    }

    [RelayCommand]
    private void NewTask()
    {
        EditingTaskId = null;
        DraftTitle = string.Empty;
        DraftDescription = string.Empty;
        DraftDueDate = null;
        DraftPriority = TaskPriority.Medium;
        DraftDifficulty = null;
        DraftTagsText = string.Empty;
        DraftSubtasks.Clear();
        ErrorMessage = null;
        IsEditorOpen = true;
    }

    [RelayCommand]
    private void EditTask(TaskItem task)
    {
        EditingTaskId = task.Id;
        DraftTitle = task.Title;
        DraftDescription = task.Description;
        DraftDueDate = task.DueDate is null
            ? null
            : task.DueDate.Value.ToDateTime(TimeOnly.MinValue);
        DraftPriority = task.Priority;
        DraftDifficulty = task.Difficulty;
        DraftTagsText = string.Join(", ", task.Tags);
        DraftSubtasks.Clear();
        foreach (var subtask in task.Subtasks)
        {
            DraftSubtasks.Add(new TaskSubtaskDraftViewModel(
                subtask.Id,
                subtask.Title,
                subtask.IsCompleted));
        }

        ErrorMessage = null;
        IsEditorOpen = true;
    }

    [RelayCommand]
    private void CloseEditor()
    {
        IsEditorOpen = false;
        DraftSubtasks.Clear();
    }

    [RelayCommand]
    private void AddSubtask()
    {
        if (DraftDifficulty != TaskDifficulty.Hard)
        {
            return;
        }

        DraftSubtasks.Add(new TaskSubtaskDraftViewModel(0, string.Empty, false));
    }

    [RelayCommand]
    private void RemoveSubtask(TaskSubtaskDraftViewModel subtask)
    {
        DraftSubtasks.Remove(subtask);
    }

    [RelayCommand]
    private async Task SaveTaskAsync(CancellationToken cancellationToken)
    {
        await RunBusyAsync(
            async token =>
            {
                var subtasks = DraftSubtasks
                    .Select((subtask, position) => new TaskSubtaskInput(
                        subtask.Id,
                        subtask.Title,
                        subtask.IsCompleted,
                        position))
                    .ToArray();
                var tags = ParseTags(DraftTagsText);
                DateOnly? dueDate = DraftDueDate is null
                    ? null
                    : DateOnly.FromDateTime(DraftDueDate.Value);

                if (EditingTaskId is null)
                {
                    await taskService.CreateAsync(
                        new CreateTaskRequest(
                            DraftTitle,
                            DraftDescription,
                            DraftPriority,
                            dueDate,
                            DraftDifficulty,
                            tags,
                            subtasks),
                        token);
                }
                else
                {
                    await taskService.UpdateAsync(
                        new UpdateTaskRequest(
                            EditingTaskId.Value,
                            DraftTitle,
                            DraftDescription,
                            DraftPriority,
                            dueDate,
                            DraftDifficulty,
                            tags,
                            subtasks),
                        token);
                }

                IsEditorOpen = false;
                DraftSubtasks.Clear();
                await LoadTasksAsync(token);
                await RefreshCalendarIfSelectedAsync(token);
            },
            cancellationToken);
    }

    [RelayCommand]
    private Task ToggleTaskAsync(TaskItem task, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await taskService.ToggleAsync(task.Id, token);
                await LoadTasksAsync(token);
                await RefreshCalendarIfSelectedAsync(token);
            },
            cancellationToken);
    }

    [RelayCommand]
    private Task ToggleSubtaskAsync(TaskSubtask subtask, CancellationToken cancellationToken)
    {
        var task = Tasks.FirstOrDefault(item => item.Subtasks.Any(item => item.Id == subtask.Id))
            ?? CalendarTasks.FirstOrDefault(item => item.Subtasks.Any(item => item.Id == subtask.Id));
        if (task is null)
        {
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                await taskService.ToggleSubtaskAsync(task.Id, subtask.Id, token);
                await LoadTasksAsync(token);
                await RefreshCalendarIfSelectedAsync(token);
            },
            cancellationToken);
    }

    [RelayCommand]
    private Task CyclePriorityAsync(TaskItem task, CancellationToken cancellationToken)
    {
        var next = task.Priority switch
        {
            TaskPriority.Low => TaskPriority.Medium,
            TaskPriority.Medium => TaskPriority.High,
            _ => TaskPriority.Low,
        };

        return RunBusyAsync(
            async token =>
            {
                await taskService.UpdateAsync(
                    new UpdateTaskRequest(
                        task.Id,
                        task.Title,
                        task.Description,
                        next,
                        task.DueDate,
                        task.Difficulty,
                        task.Tags,
                        task.Subtasks.Select((subtask, position) => new TaskSubtaskInput(
                            subtask.Id,
                            subtask.Title,
                            subtask.IsCompleted,
                            position)).ToArray()),
                    token);
                await LoadTasksAsync(token);
                await RefreshCalendarIfSelectedAsync(token);
            },
            cancellationToken);
    }

    [RelayCommand]
    private Task DeleteTaskAsync(TaskItem task, CancellationToken cancellationToken)
    {
        if (!confirmationDialog.Confirm($"Delete task \"{task.Title}\"?", "Something"))
        {
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                await taskService.DeleteAsync(task.Id, token);
                await LoadTasksAsync(token);
                await RefreshCalendarIfSelectedAsync(token);
            },
            cancellationToken);
    }

    [RelayCommand]
    private async Task LoadCalendarDateAsync(CancellationToken cancellationToken)
    {
        if (SelectedCalendarDate is null)
        {
            CalendarTasks.Clear();
            return;
        }

        using var linked = CreateLinkedToken(cancellationToken);
        IsCalendarBusy = true;
        ErrorMessage = null;
        try
        {
            var criteria = TaskSearchCriteria.Empty with
            {
                DueDate = DateOnly.FromDateTime(SelectedCalendarDate.Value),
            };
            var tasks = await taskService.SearchAsync(criteria, linked.Token);
            ReplaceCollection(CalendarTasks, tasks);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Tasks for this date could not be loaded.");
        }
        finally
        {
            IsCalendarBusy = false;
        }
    }

    partial void OnDraftDifficultyChanged(TaskDifficulty? value)
    {
        if (value != TaskDifficulty.Hard && DraftSubtasks.Count > 0)
        {
            DraftSubtasks.Clear();
        }
    }

    private async Task LoadTasksAsync(CancellationToken cancellationToken)
    {
        var criteria = new TaskSearchCriteria(
            SearchText,
            FilterDueDate is null ? null : DateOnly.FromDateTime(FilterDueDate.Value),
            ParsePriority(SelectedPriorityFilter),
            ParseDifficulty(SelectedDifficultyFilter),
            string.Equals(SelectedDifficultyFilter, "unset", StringComparison.OrdinalIgnoreCase),
            ParseTags(FilterTagsText));
        var tasks = await taskService.SearchAsync(criteria, cancellationToken);
        ReplaceCollection(Tasks, tasks);
        IsEmpty = Tasks.Count == 0;
    }

    private Task RefreshCalendarIfSelectedAsync(CancellationToken cancellationToken)
    {
        return SelectedCalendarDate is null
            ? Task.CompletedTask
            : LoadCalendarDateAsync(cancellationToken);
    }

    private async Task RunBusyAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            await operation(linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Tasks could not be updated. Please try again.");
        }
        finally
        {
            IsBusy = false;
        }
    }

    private CancellationTokenSource CreateLinkedToken(CancellationToken cancellationToken)
    {
        return _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(
                cancellationToken,
                _pageCancellation.Token);
    }

    private void HandleError(Exception exception, string fallbackMessage)
    {
        logger.LogError(exception, "Task operation failed.");
        ErrorMessage = exception is ValidationException or NotFoundException or ConflictException
            ? exception.Message
            : fallbackMessage;
    }

    private static IReadOnlyList<string> ParseTags(string value)
    {
        return value
            .Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries)
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .ToArray();
    }

    private static TaskPriority? ParsePriority(string value)
    {
        return Enum.TryParse<TaskPriority>(value, true, out var priority)
            ? priority
            : null;
    }

    private static TaskDifficulty? ParseDifficulty(string value)
    {
        return Enum.TryParse<TaskDifficulty>(value, true, out var difficulty)
            ? difficulty
            : null;
    }

    private static void ReplaceCollection<T>(
        ObservableCollection<T> destination,
        IEnumerable<T> values)
    {
        destination.Clear();
        foreach (var value in values)
        {
            destination.Add(value);
        }
    }
}
