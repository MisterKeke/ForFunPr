using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Features.Bookmarks;
using Something.Application.Messages;
using Something.Desktop.Navigation;
using Something.Domain.Exceptions;
using Something.Domain.Models.Bookmarks;

namespace Something.Desktop.ViewModels;

public sealed partial class BookmarksViewModel : PageViewModel
{
    private static readonly TimeSpan SearchDelay = TimeSpan.FromMilliseconds(250);
    private readonly IBookmarkService _bookmarkService;
    private readonly IExternalUriLauncher _uriLauncher;
    private readonly IConfirmationDialog _confirmationDialog;
    private readonly ILogger<BookmarksViewModel> _logger;
    private CancellationTokenSource? _pageCancellation;
    private CancellationTokenSource? _searchCancellation;
    private bool _mutationInProgress;

    public BookmarksViewModel(
        IBookmarkService bookmarkService,
        IExternalUriLauncher uriLauncher,
        IConfirmationDialog confirmationDialog,
        IMessenger messenger,
        ILogger<BookmarksViewModel> logger)
    {
        _bookmarkService = bookmarkService;
        _uriLauncher = uriLauncher;
        _confirmationDialog = confirmationDialog;
        _logger = logger;
        messenger.Register<BookmarksViewModel, BookmarksChangedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveBookmarksChanged(message));
    }

    public override PageKey PageKey => PageKey.Bookmarks;
    public override string Title => "Bookmarks";
    public override string Subtitle => "Save useful links locally without fetching or previewing them.";

    public ObservableCollection<BookmarkItem> Bookmarks { get; } = [];
    public ObservableCollection<string> TagOptions { get; } = ["All tags"];

    public bool CanLoadMore => Bookmarks.Count < TotalBookmarks;
    public string TotalLabel => TotalBookmarks == 1 ? "1 bookmark" : $"{TotalBookmarks} bookmarks";
    public string EditorHeading => EditingBookmark is null ? "Add bookmark" : "Edit bookmark";

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private string _selectedStatus = "All";

    [ObservableProperty]
    private string _selectedTag = "All tags";

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(TotalLabel))]
    [NotifyPropertyChangedFor(nameof(CanLoadMore))]
    private int _totalBookmarks;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(EditorHeading))]
    private BookmarkItem? _editingBookmark;

    [ObservableProperty]
    private bool _isEditorOpen;

    [ObservableProperty]
    private string _draftUrl = string.Empty;

    [ObservableProperty]
    private string _draftTitle = string.Empty;

    [ObservableProperty]
    private string _draftDescription = string.Empty;

    [ObservableProperty]
    private string _draftTags = string.Empty;

    [ObservableProperty]
    private bool _hasConflict;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        CancelAndDispose(ref _pageCancellation);
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await RunBusyAsync(LoadAllAsync, _pageCancellation.Token, "Bookmarks could not be loaded.");
    }

    public override void OnNavigatedFrom()
    {
        CancelAndDispose(ref _searchCancellation);
        CancelAndDispose(ref _pageCancellation);
        IsEditorOpen = false;
    }

    [RelayCommand]
    private Task RefreshAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(LoadAllAsync, cancellationToken, "Bookmarks could not be loaded.");
    }

    [RelayCommand]
    private Task LoadMoreAsync(CancellationToken cancellationToken)
    {
        return CanLoadMore
            ? RunBusyAsync(
                token => LoadBookmarksCoreAsync(true, token),
                cancellationToken,
                "More bookmarks could not be loaded.")
            : Task.CompletedTask;
    }

    [RelayCommand]
    private void NewBookmark()
    {
        EditingBookmark = null;
        DraftUrl = string.Empty;
        DraftTitle = string.Empty;
        DraftDescription = string.Empty;
        DraftTags = string.Empty;
        HasConflict = false;
        ErrorMessage = null;
        IsEditorOpen = true;
    }

    [RelayCommand]
    private Task EditBookmarkAsync(BookmarkItem bookmark, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var current = await _bookmarkService.GetAsync(bookmark.Id, token);
                EditingBookmark = current;
                DraftUrl = current.Url;
                DraftTitle = current.Title;
                DraftDescription = current.Description;
                DraftTags = string.Join(", ", current.Tags);
                HasConflict = false;
                IsEditorOpen = true;
            },
            cancellationToken,
            "Bookmark could not be loaded.");
    }

    [RelayCommand]
    private void CloseEditor()
    {
        IsEditorOpen = false;
        EditingBookmark = null;
        HasConflict = false;
        ErrorMessage = null;
    }

    [RelayCommand]
    private Task SaveBookmarkAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var tags = ParseTags(DraftTags);
                _mutationInProgress = true;
                try
                {
                    if (EditingBookmark is null)
                    {
                        await _bookmarkService.CreateAsync(
                            new CreateBookmarkRequest(
                                DraftUrl, DraftTitle, DraftDescription, tags),
                            token);
                    }
                    else
                    {
                        await _bookmarkService.UpdateAsync(
                            new UpdateBookmarkRequest(
                                EditingBookmark.Id,
                                DraftUrl,
                                DraftTitle,
                                DraftDescription,
                                tags,
                                EditingBookmark.Revision),
                            token);
                    }
                }
                finally
                {
                    _mutationInProgress = false;
                }

                IsEditorOpen = false;
                EditingBookmark = null;
                await LoadAllAsync(token);
            },
            cancellationToken,
            "Bookmark could not be saved.");
    }

    [RelayCommand]
    private Task ToggleReadAsync(BookmarkItem bookmark, CancellationToken cancellationToken)
    {
        return ChangeReadStateAsync(bookmark, !bookmark.IsRead, cancellationToken);
    }

    [RelayCommand]
    private async Task OpenBookmarkAsync(BookmarkItem bookmark, CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        ErrorMessage = null;
        try
        {
            await _uriLauncher.OpenAsync(bookmark.Url, linked.Token);
            if (!bookmark.IsRead)
            {
                await ChangeReadStateAsync(bookmark, true, linked.Token);
            }
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Bookmark could not be opened.");
        }
    }

    [RelayCommand]
    private Task DeleteBookmarkAsync(BookmarkItem bookmark, CancellationToken cancellationToken)
    {
        if (!_confirmationDialog.Confirm($"Delete bookmark \"{bookmark.Title}\"?", "Something"))
        {
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                _mutationInProgress = true;
                try
                {
                    await _bookmarkService.DeleteAsync(bookmark.Id, token);
                }
                finally
                {
                    _mutationInProgress = false;
                }

                await LoadAllAsync(token);
            },
            cancellationToken,
            "Bookmark could not be deleted.");
    }

    [RelayCommand]
    private Task ReloadEditorAsync(CancellationToken cancellationToken)
    {
        return EditingBookmark is null
            ? Task.CompletedTask
            : EditBookmarkAsync(EditingBookmark, cancellationToken);
    }

    partial void OnSearchTextChanged(string value) => ScheduleSearch();
    partial void OnSelectedStatusChanged(string value) => ScheduleSearch(immediate: true);
    partial void OnSelectedTagChanged(string value) => ScheduleSearch(immediate: true);

    private async Task ChangeReadStateAsync(
        BookmarkItem bookmark,
        bool isRead,
        CancellationToken cancellationToken)
    {
        await RunBusyAsync(
            async token =>
            {
                _mutationInProgress = true;
                try
                {
                    await _bookmarkService.SetReadAsync(
                        new SetBookmarkReadRequest(bookmark.Id, isRead, bookmark.Revision),
                        token);
                }
                finally
                {
                    _mutationInProgress = false;
                }

                await LoadBookmarksCoreAsync(false, token);
            },
            cancellationToken,
            isRead
                ? "The bookmark opened, but its read state could not be saved."
                : "Bookmark read state could not be changed.");
    }

    private async Task LoadAllAsync(CancellationToken cancellationToken)
    {
        await LoadBookmarksCoreAsync(false, cancellationToken);
        await LoadTagsCoreAsync(cancellationToken);
    }

    private async Task LoadBookmarksCoreAsync(bool append, CancellationToken cancellationToken)
    {
        var tags = SelectedTag == "All tags" ? [] : new[] { SelectedTag };
        var result = await _bookmarkService.ListAsync(
            new BookmarkFilter(
                SearchText,
                ParseStatus(SelectedStatus),
                tags,
                50,
                append ? Bookmarks.Count : 0),
            cancellationToken);

        if (!append)
        {
            Bookmarks.Clear();
        }

        foreach (var bookmark in result.Bookmarks)
        {
            Bookmarks.Add(bookmark);
        }

        TotalBookmarks = result.Total;
        IsEmpty = Bookmarks.Count == 0;
        OnPropertyChanged(nameof(CanLoadMore));
    }

    private async Task LoadTagsCoreAsync(CancellationToken cancellationToken)
    {
        var selected = SelectedTag;
        var tags = await _bookmarkService.ListTagsAsync(cancellationToken);
        TagOptions.Clear();
        TagOptions.Add("All tags");
        foreach (var tag in tags)
        {
            TagOptions.Add(tag);
        }

        SelectedTag = TagOptions.Contains(selected) ? selected : "All tags";
    }

    private void ScheduleSearch(bool immediate = false)
    {
        CancelAndDispose(ref _searchCancellation);
        if (_pageCancellation is null)
        {
            return;
        }

        _searchCancellation = CancellationTokenSource.CreateLinkedTokenSource(_pageCancellation.Token);
        _ = DebouncedSearchAsync(immediate, _searchCancellation.Token);
    }

    private async Task DebouncedSearchAsync(bool immediate, CancellationToken cancellationToken)
    {
        try
        {
            if (!immediate)
            {
                await Task.Delay(SearchDelay, cancellationToken);
            }

            await RunBusyAsync(
                token => LoadBookmarksCoreAsync(false, token),
                cancellationToken,
                "Bookmarks could not be loaded.");
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
    }

    private void ReceiveBookmarksChanged(BookmarksChangedMessage message)
    {
        _ = message;
        if (_mutationInProgress || _pageCancellation is null)
        {
            return;
        }

        if (IsEditorOpen)
        {
            HasConflict = true;
            ErrorMessage = "Bookmarks changed elsewhere. Reload this editor before saving.";
        }

        _ = ReloadExternalAsync(_pageCancellation.Token);
    }

    private async Task ReloadExternalAsync(CancellationToken cancellationToken)
    {
        try
        {
            await LoadAllAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Bookmarks could not be refreshed.");
        }
    }

    private async Task RunBusyAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken,
        string fallbackMessage)
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
        catch (ConflictException exception)
        {
            _logger.LogWarning(exception, "Bookmark operation encountered a revision conflict.");
            HasConflict = IsEditorOpen;
            ErrorMessage = exception.Message;
        }
        catch (Exception exception)
        {
            HandleError(exception, fallbackMessage);
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
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);
    }

    private void HandleError(Exception exception, string fallbackMessage)
    {
        _logger.LogError(exception, "Bookmark operation failed.");
        ErrorMessage = exception is ValidationException or NotFoundException or ConflictException
            ? exception.Message
            : fallbackMessage;
    }

    private static BookmarkReadStatus ParseStatus(string value)
    {
        return Enum.TryParse<BookmarkReadStatus>(value, true, out var status)
            ? status
            : BookmarkReadStatus.All;
    }

    private static IReadOnlyList<string> ParseTags(string value)
    {
        return value
            .Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .ToArray();
    }

    private static void CancelAndDispose(ref CancellationTokenSource? source)
    {
        source?.Cancel();
        source?.Dispose();
        source = null;
    }
}
