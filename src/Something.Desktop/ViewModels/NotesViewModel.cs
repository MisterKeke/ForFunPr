using System.Collections.ObjectModel;
using System.Windows;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Notes;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Messages;
using Something.Desktop.Navigation;
using Something.Domain.Exceptions;
using Something.Domain.Models.Notes;

namespace Something.Desktop.ViewModels;

public sealed partial class NotesViewModel : PageViewModel
{
    private static readonly TimeSpan AutosaveDelay = TimeSpan.FromMilliseconds(800);
    private static readonly TimeSpan SearchDelay = TimeSpan.FromMilliseconds(250);

    private readonly INoteService _noteService;
    private readonly ILogger<NotesViewModel> _logger;
    private readonly IConfirmationDialog _confirmationDialog;
    private readonly SemaphoreSlim _saveGate = new(1, 1);
    private CancellationTokenSource? _pageCancellation;
    private CancellationTokenSource? _autosaveCancellation;
    private CancellationTokenSource? _searchCancellation;
    private bool _suppressDraftChanges;
    private bool _mutationInProgress;

    public NotesViewModel(
        INoteService noteService,
        IConfirmationDialog confirmationDialog,
        IMessenger messenger,
        ILogger<NotesViewModel> logger)
    {
        _noteService = noteService;
        _confirmationDialog = confirmationDialog;
        _logger = logger;
        messenger.Register<NotesViewModel, NotesChangedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveNotesChanged(message));
    }

    public override PageKey PageKey => PageKey.Notes;
    public override string Title => "Notes";
    public override string Subtitle => "Capture ideas and keep important notes close at hand.";

    public ObservableCollection<NoteSummary> Notes { get; } = [];

    public bool CanLoadMore => Notes.Count < TotalNotes;
    public bool CanDelete => ActiveNote is not null;
    public string TotalLabel => TotalNotes == 1 ? "1 note" : $"{TotalNotes} notes";
    public string PinActionText => ActiveNote?.IsPinned == true ? "Unpin" : "Pin";
    public string ArchiveActionText => ActiveNote?.IsArchived == true ? "Restore" : "Archive";
    public string EditorTimestamp => ActiveNote is null
        ? "New note"
        : $"Updated {ActiveNote.UpdatedAt.LocalDateTime:g}";

    [ObservableProperty]
    private string _searchText = string.Empty;

    [ObservableProperty]
    private string _selectedArchiveFilter = "Active";

    [ObservableProperty]
    private bool _showPinnedOnly;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(TotalLabel))]
    [NotifyPropertyChangedFor(nameof(CanLoadMore))]
    private int _totalNotes;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CanDelete))]
    [NotifyPropertyChangedFor(nameof(PinActionText))]
    [NotifyPropertyChangedFor(nameof(ArchiveActionText))]
    [NotifyPropertyChangedFor(nameof(EditorTimestamp))]
    private NoteItem? _activeNote;

    [ObservableProperty]
    private bool _isEditorVisible;

    [ObservableProperty]
    private string _draftTitle = string.Empty;

    [ObservableProperty]
    private string _draftBody = string.Empty;

    [ObservableProperty]
    private bool _isDirty;

    [ObservableProperty]
    private bool _hasConflict;

    [ObservableProperty]
    private string _saveStatus = "Not saved";

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        CancelAndDispose(ref _pageCancellation);
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await RunBusyAsync(token => LoadNotesCoreAsync(false, token), _pageCancellation.Token);
        if (IsDirty && !HasConflict)
        {
            ScheduleAutosave();
        }
    }

    public override void OnNavigatedFrom()
    {
        CancelAndDispose(ref _autosaveCancellation);
        CancelAndDispose(ref _searchCancellation);
        CancelAndDispose(ref _pageCancellation);
    }

    [RelayCommand]
    private Task RefreshAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(token => LoadNotesCoreAsync(false, token), cancellationToken);
    }

    [RelayCommand]
    private Task LoadMoreAsync(CancellationToken cancellationToken)
    {
        return CanLoadMore
            ? RunBusyAsync(token => LoadNotesCoreAsync(true, token), cancellationToken)
            : Task.CompletedTask;
    }

    [RelayCommand]
    private async Task SelectNoteAsync(NoteSummary summary, CancellationToken cancellationToken)
    {
        if (ActiveNote?.Id == summary.Id && IsEditorVisible)
        {
            return;
        }

        if (!await FlushNoteSaveCoreAsync(cancellationToken))
        {
            return;
        }

        await RunBusyAsync(
            async token => SetEditorNote(await _noteService.GetAsync(summary.Id, token)),
            cancellationToken,
            "Note could not be loaded.");
    }

    [RelayCommand]
    private async Task NewNoteAsync(CancellationToken cancellationToken)
    {
        if (!await FlushNoteSaveCoreAsync(cancellationToken))
        {
            return;
        }

        CancelAndDispose(ref _autosaveCancellation);
        _suppressDraftChanges = true;
        ActiveNote = null;
        DraftTitle = string.Empty;
        DraftBody = string.Empty;
        IsDirty = false;
        HasConflict = false;
        SaveStatus = "Not saved";
        IsEditorVisible = true;
        _suppressDraftChanges = false;
    }

    [RelayCommand]
    private Task<bool> FlushNoteSaveAsync(CancellationToken cancellationToken)
    {
        return FlushNoteSaveCoreAsync(cancellationToken);
    }

    [RelayCommand]
    private Task TogglePinnedAsync(CancellationToken cancellationToken)
    {
        return ChangeStateAsync(isPinned: true, cancellationToken);
    }

    [RelayCommand]
    private Task ToggleArchivedAsync(CancellationToken cancellationToken)
    {
        return ChangeStateAsync(isPinned: false, cancellationToken);
    }

    [RelayCommand]
    private async Task DeleteNoteAsync(CancellationToken cancellationToken)
    {
        if (ActiveNote is null)
        {
            return;
        }

        if (!_confirmationDialog.Confirm($"Delete note \"{ActiveNote.Title}\"?", "Something"))
        {
            return;
        }

        var id = ActiveNote.Id;
        await RunBusyAsync(
            async token =>
            {
                _mutationInProgress = true;
                try
                {
                    await _noteService.DeleteAsync(id, token);
                }
                finally
                {
                    _mutationInProgress = false;
                }

                ClearEditor();
                await LoadNotesCoreAsync(false, token);
            },
            cancellationToken,
            "Note could not be deleted.");
    }

    [RelayCommand]
    private async Task ReloadActiveNoteAsync(CancellationToken cancellationToken)
    {
        if (ActiveNote is null)
        {
            return;
        }

        await RunBusyAsync(
            async token => SetEditorNote(await _noteService.GetAsync(ActiveNote.Id, token)),
            cancellationToken,
            "The latest note could not be loaded.");
    }

    partial void OnDraftTitleChanged(string value) => MarkDirtyAndScheduleAutosave();
    partial void OnDraftBodyChanged(string value) => MarkDirtyAndScheduleAutosave();
    partial void OnSearchTextChanged(string value) => ScheduleSearch();
    partial void OnSelectedArchiveFilterChanged(string value) => ScheduleSearch(immediate: true);
    partial void OnShowPinnedOnlyChanged(bool value) => ScheduleSearch(immediate: true);

    private void MarkDirtyAndScheduleAutosave()
    {
        if (_suppressDraftChanges || !IsEditorVisible)
        {
            return;
        }

        IsDirty = true;
        HasConflict = false;
        SaveStatus = "Unsaved";
        ScheduleAutosave();
    }

    private void ScheduleAutosave()
    {
        CancelAndDispose(ref _autosaveCancellation);
        if (_pageCancellation is null || HasConflict)
        {
            return;
        }

        _autosaveCancellation = CancellationTokenSource.CreateLinkedTokenSource(_pageCancellation.Token);
        _ = DebouncedAutosaveAsync(_autosaveCancellation.Token);
    }

    private async Task DebouncedAutosaveAsync(CancellationToken cancellationToken)
    {
        try
        {
            await Task.Delay(AutosaveDelay, cancellationToken);
            await FlushNoteSaveCoreAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
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

            await RunBusyAsync(token => LoadNotesCoreAsync(false, token), cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
    }

    private async Task<bool> FlushNoteSaveCoreAsync(CancellationToken cancellationToken)
    {
        CancelAndDispose(ref _autosaveCancellation);
        if (!IsDirty)
        {
            return true;
        }

        if (HasConflict)
        {
            return false;
        }

        if (string.IsNullOrWhiteSpace(DraftTitle) && string.IsNullOrWhiteSpace(DraftBody))
        {
            IsDirty = false;
            SaveStatus = "Not saved";
            return true;
        }

        using var linked = CreateLinkedToken(cancellationToken);
        await _saveGate.WaitAsync(linked.Token);
        try
        {
            if (!IsDirty || HasConflict)
            {
                return !HasConflict;
            }

            var title = DraftTitle;
            var body = DraftBody;
            SaveStatus = "Saving…";
            ErrorMessage = null;
            NoteItem saved;
            _mutationInProgress = true;
            try
            {
                saved = ActiveNote is null
                    ? await _noteService.CreateAsync(new CreateNoteRequest(title, body), linked.Token)
                    : await _noteService.UpdateAsync(
                        new UpdateNoteRequest(ActiveNote.Id, title, body, ActiveNote.Revision),
                        linked.Token);
            }
            finally
            {
                _mutationInProgress = false;
            }

            ActiveNote = saved;
            IsDirty = !string.Equals(DraftTitle, title, StringComparison.Ordinal) ||
                      !string.Equals(DraftBody, body, StringComparison.Ordinal);
            HasConflict = false;
            SaveStatus = IsDirty ? "Unsaved" : "Saved";
            IsEditorVisible = true;
            await LoadNotesCoreAsync(false, linked.Token);
            if (IsDirty)
            {
                ScheduleAutosave();
            }

            return true;
        }
        catch (ConflictException exception)
        {
            _logger.LogWarning(exception, "Note autosave encountered a revision conflict.");
            HasConflict = true;
            SaveStatus = "Conflict";
            ErrorMessage = "This note changed elsewhere. Reload the latest version before saving again.";
            return false;
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
            return false;
        }
        catch (Exception exception)
        {
            HandleError(exception, "Note could not be saved.");
            SaveStatus = "Save failed";
            return false;
        }
        finally
        {
            _saveGate.Release();
        }
    }

    private async Task ChangeStateAsync(bool isPinned, CancellationToken cancellationToken)
    {
        if (!await FlushNoteSaveCoreAsync(cancellationToken) || ActiveNote is null)
        {
            return;
        }

        await RunBusyAsync(
            async token =>
            {
                var request = new SetNoteStateRequest(
                    ActiveNote.Id,
                    isPinned ? !ActiveNote.IsPinned : !ActiveNote.IsArchived,
                    ActiveNote.Revision);
                _mutationInProgress = true;
                try
                {
                    ActiveNote = isPinned
                        ? await _noteService.SetPinnedAsync(request, token)
                        : await _noteService.SetArchivedAsync(request, token);
                }
                finally
                {
                    _mutationInProgress = false;
                }

                await LoadNotesCoreAsync(false, token);
                if (!isPinned && !MatchesArchiveFilter(ActiveNote))
                {
                    ClearEditor();
                }
            },
            cancellationToken,
            "Note state could not be changed.");
    }

    private async Task LoadNotesCoreAsync(bool append, CancellationToken cancellationToken)
    {
        var result = await _noteService.ListAsync(
            new NoteListFilter(
                SearchText,
                ParseArchiveStatus(SelectedArchiveFilter),
                ShowPinnedOnly ? true : null,
                50,
                append ? Notes.Count : 0),
            cancellationToken);

        if (!append)
        {
            Notes.Clear();
        }

        foreach (var note in result.Notes)
        {
            Notes.Add(note);
        }

        TotalNotes = result.Total;
        IsEmpty = Notes.Count == 0;
        OnPropertyChanged(nameof(CanLoadMore));
    }

    private void SetEditorNote(NoteItem note)
    {
        CancelAndDispose(ref _autosaveCancellation);
        _suppressDraftChanges = true;
        ActiveNote = note;
        DraftTitle = note.Title;
        DraftBody = note.Body;
        IsDirty = false;
        HasConflict = false;
        SaveStatus = "Saved";
        ErrorMessage = null;
        IsEditorVisible = true;
        _suppressDraftChanges = false;
    }

    private void ClearEditor()
    {
        CancelAndDispose(ref _autosaveCancellation);
        _suppressDraftChanges = true;
        ActiveNote = null;
        DraftTitle = string.Empty;
        DraftBody = string.Empty;
        IsDirty = false;
        HasConflict = false;
        SaveStatus = "Not saved";
        IsEditorVisible = false;
        _suppressDraftChanges = false;
    }

    private void ReceiveNotesChanged(NotesChangedMessage message)
    {
        _ = message;
        if (_mutationInProgress || _pageCancellation is null)
        {
            return;
        }

        var dispatcher = System.Windows.Application.Current?.Dispatcher;
        if (dispatcher is not null && !dispatcher.CheckAccess())
        {
            _ = dispatcher.InvokeAsync(() => ReceiveNotesChanged(message));
            return;
        }

        if (IsDirty && ActiveNote is not null)
        {
            CancelAndDispose(ref _autosaveCancellation);
            HasConflict = true;
            SaveStatus = "External change";
            ErrorMessage = "Notes changed elsewhere. Reload this note before saving.";
        }

        _ = ReloadAfterExternalChangeAsync(_pageCancellation.Token);
    }

    private async Task ReloadAfterExternalChangeAsync(CancellationToken cancellationToken)
    {
        try
        {
            await LoadNotesCoreAsync(false, cancellationToken);
            if (!IsDirty && ActiveNote is not null)
            {
                SetEditorNote(await _noteService.GetAsync(ActiveNote.Id, cancellationToken));
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Notes could not be refreshed.");
        }
    }

    private async Task RunBusyAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken,
        string fallbackMessage = "Notes could not be loaded.")
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
        _logger.LogError(exception, "Note operation failed.");
        ErrorMessage = exception is ValidationException or NotFoundException or ConflictException
            ? exception.Message
            : fallbackMessage;
    }

    private bool MatchesArchiveFilter(NoteItem? note)
    {
        return note is not null && ParseArchiveStatus(SelectedArchiveFilter) switch
        {
            NoteArchiveStatus.Active => !note.IsArchived,
            NoteArchiveStatus.Archived => note.IsArchived,
            _ => true,
        };
    }

    private static NoteArchiveStatus ParseArchiveStatus(string value)
    {
        return Enum.TryParse<NoteArchiveStatus>(value, true, out var status)
            ? status
            : NoteArchiveStatus.Active;
    }

    private static void CancelAndDispose(ref CancellationTokenSource? source)
    {
        source?.Cancel();
        source?.Dispose();
        source = null;
    }
}
