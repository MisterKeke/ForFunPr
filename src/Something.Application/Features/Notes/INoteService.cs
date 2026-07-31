using Something.Domain.Models.Notes;

namespace Something.Application.Features.Notes;

public interface INoteService
{
    Task<NoteListResult> ListAsync(NoteListFilter? filter = null, CancellationToken cancellationToken = default);
    Task<NoteItem> GetAsync(long id, CancellationToken cancellationToken = default);
    Task<NoteItem> CreateAsync(CreateNoteRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> UpdateAsync(UpdateNoteRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> SetPinnedAsync(SetNoteStateRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> SetArchivedAsync(SetNoteStateRequest request, CancellationToken cancellationToken = default);
    Task DeleteAsync(long id, CancellationToken cancellationToken = default);
}
