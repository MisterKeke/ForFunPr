using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Messages;
using Something.Domain.Models.Notes;
using Something.Domain.Validation;

namespace Something.Application.Features.Notes;

public sealed class NoteService(INoteRepository repository, IMessenger messenger) : INoteService
{
    public Task<NoteListResult> ListAsync(
        NoteListFilter? filter = null,
        CancellationToken cancellationToken = default)
    {
        filter ??= NoteListFilter.Default;
        var query = NoteRules.NormalizeSearch(filter.Query);
        var (limit, offset) = NoteRules.NormalizePage(filter.Limit, filter.Offset);
        return repository.ListAsync(
            filter with { Query = query, Limit = limit, Offset = offset },
            cancellationToken);
    }

    public Task<NoteItem> GetAsync(long id, CancellationToken cancellationToken = default)
    {
        NoteRules.RequirePositiveId(id);
        return repository.GetAsync(id, cancellationToken);
    }

    public async Task<NoteItem> CreateAsync(
        CreateNoteRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        var (title, body) = NoteRules.NormalizeWrite(request.Title, request.Body);
        var note = await repository.CreateAsync(request with { Title = title, Body = body }, cancellationToken);
        PublishChanged();
        return note;
    }

    public async Task<NoteItem> UpdateAsync(
        UpdateNoteRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        ValidateMutation(request.Id, request.ExpectedRevision);
        var (title, body) = NoteRules.NormalizeWrite(request.Title, request.Body);
        var note = await repository.UpdateAsync(request with { Title = title, Body = body }, cancellationToken);
        PublishChanged();
        return note;
    }

    public async Task<NoteItem> SetPinnedAsync(
        SetNoteStateRequest request,
        CancellationToken cancellationToken = default)
    {
        ValidateStateRequest(request);
        var note = await repository.SetPinnedAsync(request, cancellationToken);
        PublishChanged();
        return note;
    }

    public async Task<NoteItem> SetArchivedAsync(
        SetNoteStateRequest request,
        CancellationToken cancellationToken = default)
    {
        ValidateStateRequest(request);
        var note = await repository.SetArchivedAsync(request, cancellationToken);
        PublishChanged();
        return note;
    }

    public async Task DeleteAsync(long id, CancellationToken cancellationToken = default)
    {
        NoteRules.RequirePositiveId(id);
        await repository.DeleteAsync(id, cancellationToken);
        PublishChanged();
    }

    private static void ValidateStateRequest(SetNoteStateRequest request)
    {
        ArgumentNullException.ThrowIfNull(request);
        ValidateMutation(request.Id, request.ExpectedRevision);
    }

    private static void ValidateMutation(long id, int revision)
    {
        NoteRules.RequirePositiveId(id);
        NoteRules.RequirePositiveRevision(revision);
    }

    private void PublishChanged() => messenger.Send(new NotesChangedMessage(DateTimeOffset.UtcNow));
}
