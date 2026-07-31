using Something.Domain.Models.Bookmarks;

namespace Something.Application.Features.Bookmarks;

public sealed record BookmarkListResult(
    IReadOnlyList<BookmarkItem> Bookmarks,
    int Total,
    int Limit,
    int Offset);
