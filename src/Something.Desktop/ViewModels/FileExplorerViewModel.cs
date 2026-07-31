using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Abstractions.FileSystem;
using Something.Desktop.Navigation;
using Something.Domain.Exceptions;
using Something.Domain.Models.FileExplorer;

namespace Something.Desktop.ViewModels;

public sealed partial class FileExplorerViewModel(
    IFileExplorerService explorer,
    IFolderPicker folderPicker,
    IConfirmationDialog confirmationDialog,
    ILogger<FileExplorerViewModel> logger) : PageViewModel
{
    private const int PageSize = 250;
    private const int HistoryLimit = 50;
    private static readonly TimeSpan SearchDelay = TimeSpan.FromMilliseconds(200);
    private readonly List<ExplorerLocation> _history = [];
    private CancellationTokenSource? _pageCancellation;
    private CancellationTokenSource? _searchCancellation;
    private int _historyIndex = -1;
    private FileExplorerListing? _listing;

    public override PageKey PageKey => PageKey.FileExplorer;
    public override string Title => "File Explorer";
    public override string Subtitle => "Browse approved Windows locations and safely open or recycle files.";

    public ObservableCollection<FileExplorerPlace> Places { get; } = [];
    public ObservableCollection<FileExplorerBreadcrumb> Breadcrumbs { get; } = [];
    public ObservableCollection<FileExplorerEntry> Entries { get; } = [];

    public bool CanGoBack => !IsBusy && _historyIndex > 0;
    public bool CanGoForward => !IsBusy && _historyIndex >= 0 && _historyIndex < _history.Count - 1;
    public bool CanGoUp => !IsBusy && _listing?.CanGoUp == true;
    public bool CanRefresh => !IsBusy && _listing is not null;
    public bool CanLoadMore => !IsBusy && _listing?.HasMore == true;
    public string Summary => SearchText.Trim().Length == 0
        ? $"{Entries.Count:N0} {(Entries.Count == 1 ? "item" : "items")}"
        : $"{Entries.Count:N0} matching {(Entries.Count == 1 ? "item" : "items")} shown for \"{SearchText.Trim()}\"";
    public string EmptyMessage => SearchText.Trim().Length == 0
        ? "This folder is empty."
        : $"No items match \"{SearchText.Trim()}\" in this folder.";
    public string LimitMessage => _listing?.Truncated == true
        ? $"Showing the first {_listing.MaximumItems:N0}{(SearchText.Trim().Length > 0 ? " matching" : string.Empty)} items."
        : string.Empty;
    public bool HasLimitMessage => LimitMessage.Length > 0;

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private FileExplorerPlace? _selectedPlace;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        CancelPage();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await InitializeAsync(_pageCancellation.Token);
    }

    public override void OnNavigatedFrom() => CancelPage();

    [RelayCommand(CanExecute = nameof(CanGoBack))]
    private Task BackAsync(CancellationToken cancellationToken) => MoveHistoryAsync(-1, cancellationToken);

    [RelayCommand(CanExecute = nameof(CanGoForward))]
    private Task ForwardAsync(CancellationToken cancellationToken) => MoveHistoryAsync(1, cancellationToken);

    [RelayCommand(CanExecute = nameof(CanGoUp))]
    private Task UpAsync(CancellationToken cancellationToken) =>
        _listing is null
            ? Task.CompletedTask
            : OpenLocationAsync(_listing.RootId, _listing.ParentPath, HistoryMode.Push, string.Empty, cancellationToken);

    [RelayCommand(CanExecute = nameof(CanRefresh))]
    private Task RefreshAsync(CancellationToken cancellationToken) =>
        _listing is null
            ? Task.CompletedTask
            : OpenLocationAsync(_listing.RootId, _listing.Path, HistoryMode.None, SearchText, cancellationToken);

    [RelayCommand]
    private async Task ChooseFolderAsync(CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        try
        {
            var path = await folderPicker.PickFolderAsync(linked.Token);
            if (path is null) return;
            IsBusy = true;
            ErrorMessage = null;
            var listing = await explorer.RegisterChosenRootAsync(path, linked.Token);
            ApplyListing(listing, append: false);
            RememberLocation(listing.RootId, listing.Path, HistoryMode.Push);
            Replace(Places, await explorer.GetPlacesAsync(linked.Token));
            SelectedPlace = Places.FirstOrDefault(place => place.RootId == listing.RootId);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "The folder could not be added.");
        }
        finally
        {
            IsBusy = false;
            RaiseControlState();
        }
    }

    [RelayCommand]
    private Task OpenPlaceAsync(FileExplorerPlace place, CancellationToken cancellationToken) =>
        OpenLocationAsync(place.RootId, string.Empty, HistoryMode.Push, string.Empty, cancellationToken);

    [RelayCommand]
    private Task OpenBreadcrumbAsync(FileExplorerBreadcrumb breadcrumb, CancellationToken cancellationToken) =>
        _listing is null
            ? Task.CompletedTask
            : OpenLocationAsync(_listing.RootId, breadcrumb.Path, HistoryMode.Push, string.Empty, cancellationToken);

    [RelayCommand]
    private Task OpenEntryAsync(FileExplorerEntry entry, CancellationToken cancellationToken)
    {
        if (entry.IsDirectory && _listing is not null)
        {
            return OpenLocationAsync(_listing.RootId, entry.Path, HistoryMode.Push, string.Empty, cancellationToken);
        }

        return entry.CanActOnFile ? OpenFileAsync(entry, cancellationToken) : Task.CompletedTask;
    }

    [RelayCommand]
    private async Task DeleteEntryAsync(FileExplorerEntry entry, CancellationToken cancellationToken)
    {
        if (_listing is null || !entry.CanActOnFile ||
            !confirmationDialog.Confirm($"Move \"{entry.Name}\" to the Recycle Bin?", "Something"))
        {
            return;
        }

        using var linked = CreateLinkedToken(cancellationToken);
        try
        {
            IsBusy = true;
            ErrorMessage = null;
            await explorer.RecycleFileAsync(_listing.RootId, entry.Path, linked.Token);
            await OpenLocationCoreAsync(
                _listing.RootId, _listing.Path, HistoryMode.None, SearchText, linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "The file could not be moved to the Recycle Bin.");
        }
        finally
        {
            IsBusy = false;
            RaiseControlState();
        }
    }

    [RelayCommand(CanExecute = nameof(CanLoadMore))]
    private async Task LoadMoreAsync(CancellationToken cancellationToken)
    {
        if (_listing is null) return;
        using var linked = CreateLinkedToken(cancellationToken);
        try
        {
            IsBusy = true;
            var listing = await explorer.ListDirectoryAsync(
                _listing.RootId, _listing.Path, SearchText,
                _listing.NextOffset, PageSize, linked.Token);
            ApplyListing(listing, append: true);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "More files could not be loaded.");
        }
        finally
        {
            IsBusy = false;
            RaiseControlState();
        }
    }

    partial void OnSearchTextChanged(string value)
    {
        _ = value;
        OnPropertyChanged(nameof(Summary));
        OnPropertyChanged(nameof(EmptyMessage));
        ScheduleSearch();
    }

    private async Task InitializeAsync(CancellationToken cancellationToken)
    {
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            var places = await explorer.GetPlacesAsync(cancellationToken);
            Replace(Places, places);
            var initial = places.FirstOrDefault(place => place.Kind == "home") ?? places.FirstOrDefault();
            if (initial is null)
            {
                throw new InvalidOperationException("No folders are available to browse.");
            }

            await OpenLocationCoreAsync(initial.RootId, string.Empty, HistoryMode.Replace, string.Empty, cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "File Explorer could not be loaded.");
        }
        finally
        {
            IsBusy = false;
            RaiseControlState();
        }
    }

    private async Task OpenLocationAsync(
        string rootId,
        string path,
        HistoryMode historyMode,
        string query,
        CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            await OpenLocationCoreAsync(rootId, path, historyMode, query, linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "That folder could not be loaded.");
        }
        finally
        {
            IsBusy = false;
            RaiseControlState();
        }
    }

    private async Task OpenLocationCoreAsync(
        string rootId,
        string path,
        HistoryMode historyMode,
        string query,
        CancellationToken cancellationToken)
    {
        var listing = await explorer.ListDirectoryAsync(
            rootId, path, query, 0, PageSize, cancellationToken);
        ApplyListing(listing, append: false);
        RememberLocation(listing.RootId, listing.Path, historyMode);
        SelectedPlace = Places.FirstOrDefault(place => place.RootId == listing.RootId);
    }

    private async Task OpenFileAsync(FileExplorerEntry entry, CancellationToken cancellationToken)
    {
        if (_listing is null) return;
        using var linked = CreateLinkedToken(cancellationToken);
        try
        {
            ErrorMessage = null;
            await explorer.OpenFileAsync(_listing.RootId, entry.Path, linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "The file could not be opened.");
        }
    }

    private async Task MoveHistoryAsync(int direction, CancellationToken cancellationToken)
    {
        var nextIndex = _historyIndex + direction;
        if (nextIndex < 0 || nextIndex >= _history.Count) return;
        var destination = _history[nextIndex];
        await OpenLocationAsync(
            destination.RootId, destination.Path, HistoryMode.None, string.Empty, cancellationToken);
        if (_listing?.RootId == destination.RootId && _listing.Path == destination.Path)
        {
            _historyIndex = nextIndex;
            RaiseControlState();
        }
    }

    private void ApplyListing(FileExplorerListing listing, bool append)
    {
        _listing = listing;
        if (!string.Equals(SearchText, listing.Query, StringComparison.Ordinal))
        {
            SearchText = listing.Query;
        }

        if (!append) Entries.Clear();
        foreach (var entry in listing.Entries)
        {
            if (!Entries.Any(existing => existing.Path.Equals(entry.Path, StringComparison.OrdinalIgnoreCase)))
            {
                Entries.Add(entry);
            }
        }

        var sorted = Entries.OrderBy(entry => entry.IsDirectory ? 0 : 1)
            .ThenBy(entry => entry.Name, StringComparer.CurrentCultureIgnoreCase).ToArray();
        Replace(Entries, sorted);
        Replace(Breadcrumbs, listing.Breadcrumbs);
        IsEmpty = Entries.Count == 0;
        OnPropertyChanged(nameof(Summary));
        OnPropertyChanged(nameof(EmptyMessage));
        OnPropertyChanged(nameof(LimitMessage));
        OnPropertyChanged(nameof(HasLimitMessage));
        RaiseControlState();
    }

    private void RememberLocation(string rootId, string path, HistoryMode mode)
    {
        if (mode == HistoryMode.None) return;
        var location = new ExplorerLocation(rootId, path);
        if (mode == HistoryMode.Replace)
        {
            _history.Clear();
            _history.Add(location);
            _historyIndex = 0;
            return;
        }

        if (_historyIndex >= 0 && _history[_historyIndex] == location) return;
        if (_historyIndex < _history.Count - 1)
        {
            _history.RemoveRange(_historyIndex + 1, _history.Count - _historyIndex - 1);
        }

        _history.Add(location);
        if (_history.Count > HistoryLimit) _history.RemoveAt(0);
        _historyIndex = _history.Count - 1;
    }

    private void ScheduleSearch()
    {
        _searchCancellation?.Cancel();
        _searchCancellation?.Dispose();
        if (_pageCancellation is null || _listing is null) return;
        _searchCancellation = CancellationTokenSource.CreateLinkedTokenSource(_pageCancellation.Token);
        _ = RunSearchAsync(_listing.RootId, _listing.Path, SearchText, _searchCancellation.Token);
    }

    private async Task RunSearchAsync(string rootId, string path, string query, CancellationToken cancellationToken)
    {
        try
        {
            await Task.Delay(SearchDelay, cancellationToken);
            await OpenLocationAsync(rootId, path, HistoryMode.None, query, cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
    }

    private void RaiseControlState()
    {
        OnPropertyChanged(nameof(CanGoBack));
        OnPropertyChanged(nameof(CanGoForward));
        OnPropertyChanged(nameof(CanGoUp));
        OnPropertyChanged(nameof(CanRefresh));
        OnPropertyChanged(nameof(CanLoadMore));
        BackCommand.NotifyCanExecuteChanged();
        ForwardCommand.NotifyCanExecuteChanged();
        UpCommand.NotifyCanExecuteChanged();
        RefreshCommand.NotifyCanExecuteChanged();
        LoadMoreCommand.NotifyCanExecuteChanged();
    }

    private CancellationTokenSource CreateLinkedToken(CancellationToken cancellationToken) =>
        _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);

    private void CancelPage()
    {
        _searchCancellation?.Cancel();
        _searchCancellation?.Dispose();
        _searchCancellation = null;
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }

    private void HandleError(Exception exception, string fallback)
    {
        logger.LogError(exception, "File Explorer operation failed.");
        ErrorMessage = exception is ValidationException or InvalidOperationException
            ? exception.Message
            : fallback;
    }

    private static void Replace<T>(ObservableCollection<T> target, IEnumerable<T> source)
    {
        target.Clear();
        foreach (var item in source) target.Add(item);
    }

    private sealed record ExplorerLocation(string RootId, string Path);
    private enum HistoryMode { None, Push, Replace }
}
