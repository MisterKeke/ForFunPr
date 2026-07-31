using Something.Application.Features.Notes;
using Something.Domain.Models.Notes;

namespace Something.Application.Abstractions.Persistence;

public interface INoteRepository
{
    Task<NoteListResult> ListAsync(NoteListFilter filter, CancellationToken cancellationToken = default);
    Task<NoteItem> GetAsync(long id, CancellationToken cancellationToken = default);
    Task<NoteItem> CreateAsync(CreateNoteRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> UpdateAsync(UpdateNoteRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> SetPinnedAsync(SetNoteStateRequest request, CancellationToken cancellationToken = default);
    Task<NoteItem> SetArchivedAsync(SetNoteStateRequest request, CancellationToken cancellationToken = default);
    Task DeleteAsync(long id, CancellationToken cancellationToken = default);
}
