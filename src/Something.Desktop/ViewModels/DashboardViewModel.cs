using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Features.Favorites;
using Something.Application.Features.Tasks;
using Something.Application.Messages;
using Something.Desktop.Navigation;
using Something.Domain.Models.Favorites;
using Something.Domain.Models.Tasks;

namespace Something.Desktop.ViewModels;

public sealed partial class DashboardViewModel : PageViewModel
{
    private static readonly TimeSpan AutomaticRefreshInterval = TimeSpan.FromMinutes(10);
    private readonly ITaskService _taskService;
    private readonly IFavoriteUpdateService _favoriteUpdates;
    private readonly IExternalUriLauncher _uriLauncher;
    private readonly IMessenger _messenger;
    private readonly ILogger<DashboardViewModel> _logger;
    private CancellationTokenSource? _pageCancellation;
    private bool _taskMutationInProgress;

    public DashboardViewModel(
        ITaskService taskService,
        IFavoriteUpdateService favoriteUpdates,
        IExternalUriLauncher uriLauncher,
        IMessenger messenger,
        ILogger<DashboardViewModel> logger)
    {
        _taskService = taskService;
        _favoriteUpdates = favoriteUpdates;
        _uriLauncher = uriLauncher;
        _messenger = messenger;
        _logger = logger;
        messenger.Register<DashboardViewModel, TasksChangedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveTasksChanged(message));
        messenger.Register<DashboardViewModel, FavoritesChangedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveFavoritesChanged(message));
    }

    public override PageKey PageKey => PageKey.Dashboard;
    public override string Title => "Dashboard";
    public override string Subtitle => "Your day, upcoming work, and favorite updates in one place.";

    public ObservableCollection<TaskItem> TodayTasks { get; } = [];
    public ObservableCollection<TaskItem> WeekTasks { get; } = [];
    public ObservableCollection<FavoriteUpdateItem> FavoriteUpdates { get; } = [];

    public bool HasPartialErrors => !string.IsNullOrWhiteSpace(PartialErrorMessage);
    public bool HasFavoriteUpdates => FavoriteUpdates.Count > 0;
    public string TodayCountLabel => TodayTasks.Count == 1 ? "1 task due today" : $"{TodayTasks.Count} tasks due today";
    public string WeekCountLabel => WeekTasks.Count == 1 ? "1 task remaining this week" : $"{WeekTasks.Count} tasks remaining this week";

    [ObservableProperty]
    private bool _isRefreshing;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasPartialErrors))]
    private string? _partialErrorMessage;

    [ObservableProperty]
    private string _lastRefreshedLabel = "Not refreshed yet";

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        CancelPage();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await LoadDashboardAsync(runInitialScan: true, _pageCancellation.Token);
        _ = RunAutomaticRefreshAsync(_pageCancellation.Token);
    }

    public override void OnNavigatedFrom() => CancelPage();

    [RelayCommand]
    private async Task RefreshFavoritesAsync(CancellationToken cancellationToken)
    {
        if (IsRefreshing)
        {
            return;
        }

        using var linked = CreateLinkedToken(cancellationToken);
        IsRefreshing = true;
        try
        {
            var result = await _favoriteUpdates.RefreshAsync(linked.Token);
            ApplyFavoriteResult(result);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Favorite updates could not be refreshed.");
            ErrorMessage = "Favorite updates could not be refreshed.";
        }
        finally
        {
            IsRefreshing = false;
        }
    }

    [RelayCommand]
    private async Task ToggleTaskAsync(TaskItem task, CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        _taskMutationInProgress = true;
        try
        {
            await _taskService.ToggleAsync(task.Id, linked.Token);
            await LoadTasksAsync(linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Dashboard task could not be updated.");
            ErrorMessage = "The task could not be updated.";
        }
        finally
        {
            _taskMutationInProgress = false;
        }
    }

    [RelayCommand]
    private async Task OpenUpdateAsync(FavoriteUpdateItem item, CancellationToken cancellationToken)
    {
        try
        {
            await _uriLauncher.OpenAsync(item.Url, cancellationToken);
        }
        catch (Exception exception)
        {
            _logger.LogWarning(exception, "Favorite update link could not be opened.");
            ErrorMessage = "The update link could not be opened.";
        }
    }

    [RelayCommand]
    private void OpenTasks() => _messenger.Send(new NavigateRequestedMessage(PageKey.Tasks));

    [RelayCommand]
    private void OpenTelegram() => _messenger.Send(new NavigateRequestedMessage(PageKey.Telegram));

    [RelayCommand]
    private void OpenYouTube() => _messenger.Send(new NavigateRequestedMessage(PageKey.YouTube));

    private async Task LoadDashboardAsync(bool runInitialScan, CancellationToken cancellationToken)
    {
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            var todayTask = _taskService.GetTodayIncompleteAsync(cancellationToken);
            var weekTask = _taskService.GetRemainingWeekIncompleteAsync(cancellationToken);
            var favoriteTask = runInitialScan
                ? _favoriteUpdates.GetInitialAsync(cancellationToken)
                : _favoriteUpdates.GetCurrentAsync(cancellationToken);
            await Task.WhenAll(todayTask, weekTask, favoriteTask);
            Replace(TodayTasks, await todayTask);
            Replace(WeekTasks, await weekTask);
            ApplyFavoriteResult(await favoriteTask);
            RaiseTaskSummaryProperties();
            IsEmpty = TodayTasks.Count == 0 && WeekTasks.Count == 0 && FavoriteUpdates.Count == 0;
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Dashboard could not be loaded.");
            ErrorMessage = "The dashboard could not be loaded.";
        }
        finally
        {
            IsBusy = false;
        }
    }

    private async Task LoadTasksAsync(CancellationToken cancellationToken)
    {
        var todayTask = _taskService.GetTodayIncompleteAsync(cancellationToken);
        var weekTask = _taskService.GetRemainingWeekIncompleteAsync(cancellationToken);
        await Task.WhenAll(todayTask, weekTask);
        Replace(TodayTasks, await todayTask);
        Replace(WeekTasks, await weekTask);
        RaiseTaskSummaryProperties();
    }

    private void ApplyFavoriteResult(FavoriteUpdateScanResult result)
    {
        Replace(FavoriteUpdates, result.Updates);
        OnPropertyChanged(nameof(HasFavoriteUpdates));
        PartialErrorMessage = SummarizeErrors(result.Errors);
        var refreshedAt = result.ScanStartedAt ?? result.State.LastRefreshAt ?? result.State.CurrentOpenedAt;
        LastRefreshedLabel = refreshedAt is null
            ? "Not refreshed yet"
            : $"Refreshed {refreshedAt.Value.LocalDateTime:g}";
    }

    private async Task RunAutomaticRefreshAsync(CancellationToken cancellationToken)
    {
        try
        {
            using var timer = new PeriodicTimer(AutomaticRefreshInterval);
            while (await timer.WaitForNextTickAsync(cancellationToken))
            {
                await RefreshFavoritesAsync(cancellationToken);
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Automatic favorite refresh stopped unexpectedly.");
        }
    }

    private void ReceiveTasksChanged(TasksChangedMessage message)
    {
        _ = message;
        if (_taskMutationInProgress || _pageCancellation is null)
        {
            return;
        }

        _ = ReloadTasksFromMessageAsync(_pageCancellation.Token);
    }

    private void ReceiveFavoritesChanged(FavoritesChangedMessage message)
    {
        _ = message;
        if (_pageCancellation is null)
        {
            return;
        }

        _ = ReloadFavoritesFromMessageAsync(_pageCancellation.Token);
    }

    private async Task ReloadTasksFromMessageAsync(CancellationToken cancellationToken)
    {
        try
        {
            await LoadTasksAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Dashboard task summaries could not be reloaded.");
        }
    }

    private async Task ReloadFavoritesFromMessageAsync(CancellationToken cancellationToken)
    {
        try
        {
            ApplyFavoriteResult(await _favoriteUpdates.GetInitialAsync(cancellationToken));
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            _logger.LogError(exception, "Dashboard favorites could not be reloaded.");
        }
    }

    private void RaiseTaskSummaryProperties()
    {
        OnPropertyChanged(nameof(TodayCountLabel));
        OnPropertyChanged(nameof(WeekCountLabel));
    }

    private CancellationTokenSource CreateLinkedToken(CancellationToken cancellationToken) =>
        _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);

    private void CancelPage()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }

    private static void Replace<T>(ObservableCollection<T> target, IEnumerable<T> items)
    {
        target.Clear();
        foreach (var item in items)
        {
            target.Add(item);
        }
    }

    private static string? SummarizeErrors(IReadOnlyList<FavoriteUpdateError> errors)
    {
        if (errors.Count == 0)
        {
            return null;
        }

        var sample = errors.Take(2).Select(error =>
            $"{(string.IsNullOrWhiteSpace(error.SourceId) ? error.SourceLabel() : error.SourceId)}: {error.Message}");
        var suffix = errors.Count > 2 ? $" and {errors.Count - 2} more" : string.Empty;
        return $"Some favorites could not refresh ({string.Join("; ", sample)}{suffix}).";
    }
}

file static class FavoriteUpdateErrorPresentation
{
    public static string SourceLabel(this FavoriteUpdateError error) =>
        error.Source == Something.Domain.Enums.FavoriteSource.Telegram ? "Telegram" : "YouTube";
}
