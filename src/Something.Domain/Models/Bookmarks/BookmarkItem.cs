namespace Something.Domain.Models.Bookmarks;

public sealed record BookmarkItem(
    long Id,
    string Url,
    string Title,
    string Description,
    bool IsRead,
    DateTimeOffset? ReadAt,
    IReadOnlyList<string> Tags,
    int Revision,
    DateTimeOffset CreatedAt,
    DateTimeOffset UpdatedAt)
{
    public string Host => Uri.TryCreate(Url, UriKind.Absolute, out var uri) ? uri.Host : Url;
    public string ReadActionText => IsRead ? "Mark unread" : "Mark read";
    public string ReadStateText => IsRead ? "Read" : "Unread";
}
