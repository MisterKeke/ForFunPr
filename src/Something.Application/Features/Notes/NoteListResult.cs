using Something.Domain.Models.Notes;

namespace Something.Application.Features.Notes;

public sealed record NoteListResult(
    IReadOnlyList<NoteSummary> Notes,
    int Total,
    int Limit,
    int Offset);
