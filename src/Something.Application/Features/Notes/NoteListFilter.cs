namespace Something.Application.Features.Notes;

public sealed record NoteListFilter(
    string Query,
    NoteArchiveStatus ArchiveStatus,
    bool? IsPinned,
    int Limit,
    int Offset)
{
    public static NoteListFilter Default { get; } = new(
        string.Empty,
        NoteArchiveStatus.Active,
        null,
        50,
        0);
}
