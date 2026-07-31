using Something.Domain.Validation;

namespace Something.Domain.Models.Notes;

public sealed record NoteItem(
    long Id,
    string Title,
    string Body,
    bool IsPinned,
    bool IsArchived,
    int Revision,
    DateTimeOffset CreatedAt,
    DateTimeOffset UpdatedAt)
{
    public string DisplayTitle => NoteRules.GetDisplayTitle(Title, Body);
}
