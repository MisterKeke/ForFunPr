namespace Something.Application.Features.Bookmarks;

public sealed record SetBookmarkReadRequest(long Id, bool IsRead, int ExpectedRevision);
