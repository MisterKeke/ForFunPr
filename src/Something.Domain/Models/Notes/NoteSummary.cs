using Something.Domain.Validation;

namespace Something.Domain.Models.Notes;

public sealed record NoteSummary(
    long Id,
    string Title,
    string Preview,
    bool IsPinned,
    bool IsArchived,
    int Revision,
    DateTimeOffset CreatedAt,
    DateTimeOffset UpdatedAt)
{
    public string DisplayTitle => NoteRules.GetDisplayTitle(Title, Preview);
}
