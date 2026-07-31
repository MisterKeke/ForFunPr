namespace Something.Application.Features.Bookmarks;

public sealed record CreateBookmarkRequest(
    string Url,
    string Title,
    string Description,
    IReadOnlyList<string> Tags,
    string NormalizedUrl = "");
