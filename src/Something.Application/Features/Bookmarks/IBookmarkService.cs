using Something.Domain.Models.Bookmarks;

namespace Something.Application.Features.Bookmarks;

public interface IBookmarkService
{
    Task<BookmarkListResult> ListAsync(BookmarkFilter? filter = null, CancellationToken cancellationToken = default);
    Task<BookmarkItem> GetAsync(long id, CancellationToken cancellationToken = default);
    Task<BookmarkItem> CreateAsync(CreateBookmarkRequest request, CancellationToken cancellationToken = default);
    Task<BookmarkItem> UpdateAsync(UpdateBookmarkRequest request, CancellationToken cancellationToken = default);
    Task<BookmarkItem> SetReadAsync(SetBookmarkReadRequest request, CancellationToken cancellationToken = default);
    Task DeleteAsync(long id, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<string>> ListTagsAsync(CancellationToken cancellationToken = default);
}
