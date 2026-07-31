namespace Something.Application.Features.Bookmarks;

public sealed record BookmarkFilter(
    string Query,
    BookmarkReadStatus Status,
    IReadOnlyList<string> Tags,
    int Limit,
    int Offset)
{
    public static BookmarkFilter Default { get; } = new(
        string.Empty, BookmarkReadStatus.All, [], 50, 0);
}
