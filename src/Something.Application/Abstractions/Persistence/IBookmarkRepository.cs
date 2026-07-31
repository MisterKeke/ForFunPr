using Something.Application.Features.Bookmarks;
using Something.Domain.Models.Bookmarks;

namespace Something.Application.Abstractions.Persistence;

public interface IBookmarkRepository
{
    Task<BookmarkListResult> ListAsync(BookmarkFilter filter, CancellationToken cancellationToken = default);
    Task<BookmarkItem> GetAsync(long id, CancellationToken cancellationToken = default);
    Task<BookmarkItem> CreateAsync(CreateBookmarkRequest request, CancellationToken cancellationToken = default);
    Task<BookmarkItem> UpdateAsync(UpdateBookmarkRequest request, CancellationToken cancellationToken = default);
    Task<BookmarkItem> SetReadAsync(SetBookmarkReadRequest request, CancellationToken cancellationToken = default);
    Task DeleteAsync(long id, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<string>> ListTagsAsync(CancellationToken cancellationToken = default);
}
