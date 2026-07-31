namespace Something.Application.Features.Bookmarks;

public sealed record UpdateBookmarkRequest(
    long Id,
    string Url,
    string Title,
    string Description,
    IReadOnlyList<string> Tags,
    int ExpectedRevision,
    string NormalizedUrl = "");
