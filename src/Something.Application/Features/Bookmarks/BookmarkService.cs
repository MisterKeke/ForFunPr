using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Messages;
using Something.Domain.Models.Bookmarks;
using Something.Domain.Validation;

namespace Something.Application.Features.Bookmarks;

public sealed class BookmarkService(
    IBookmarkRepository repository,
    IMessenger messenger) : IBookmarkService
{
    public Task<BookmarkListResult> ListAsync(
        BookmarkFilter? filter = null,
        CancellationToken cancellationToken = default)
    {
        filter ??= BookmarkFilter.Default;
        var query = BookmarkRules.NormalizeSearch(filter.Query);
        var tags = BookmarkRules.NormalizeTags(filter.Tags);
        var (limit, offset) = BookmarkRules.NormalizePage(filter.Limit, filter.Offset);
        return repository.ListAsync(
            filter with { Query = query, Tags = tags, Limit = limit, Offset = offset },
            cancellationToken);
    }

    public Task<BookmarkItem> GetAsync(long id, CancellationToken cancellationToken = default)
    {
        BookmarkRules.RequirePositiveId(id);
        return repository.GetAsync(id, cancellationToken);
    }

    public async Task<BookmarkItem> CreateAsync(
        CreateBookmarkRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        var normalized = Normalize(request.Url, request.Title, request.Description, request.Tags);
        var bookmark = await repository.CreateAsync(
            request with
            {
                Url = normalized.Url,
                NormalizedUrl = normalized.NormalizedUrl,
                Title = normalized.Title,
                Description = normalized.Description,
                Tags = normalized.Tags,
            },
            cancellationToken);
        PublishChanged();
        return bookmark;
    }

    public async Task<BookmarkItem> UpdateAsync(
        UpdateBookmarkRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        BookmarkRules.RequireMutation(request.Id, request.ExpectedRevision);
        var normalized = Normalize(request.Url, request.Title, request.Description, request.Tags);
        var bookmark = await repository.UpdateAsync(
            request with
            {
                Url = normalized.Url,
                NormalizedUrl = normalized.NormalizedUrl,
                Title = normalized.Title,
                Description = normalized.Description,
                Tags = normalized.Tags,
            },
            cancellationToken);
        PublishChanged();
        return bookmark;
    }

    public async Task<BookmarkItem> SetReadAsync(
        SetBookmarkReadRequest request,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        BookmarkRules.RequireMutation(request.Id, request.ExpectedRevision);
        var bookmark = await repository.SetReadAsync(request, cancellationToken);
        PublishChanged();
        return bookmark;
    }

    public async Task DeleteAsync(long id, CancellationToken cancellationToken = default)
    {
        BookmarkRules.RequirePositiveId(id);
        await repository.DeleteAsync(id, cancellationToken);
        PublishChanged();
    }

    public Task<IReadOnlyList<string>> ListTagsAsync(CancellationToken cancellationToken = default)
    {
        return repository.ListTagsAsync(cancellationToken);
    }

    private static (string Url, string NormalizedUrl, string Title, string Description, IReadOnlyList<string> Tags)
        Normalize(string url, string title, string description, IReadOnlyList<string> tags)
    {
        return BookmarkRules.NormalizeWrite(url, title, description, tags);
    }

    private void PublishChanged() => messenger.Send(new BookmarksChangedMessage(DateTimeOffset.UtcNow));
}
