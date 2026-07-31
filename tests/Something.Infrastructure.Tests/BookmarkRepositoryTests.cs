using Something.Application.Features.Bookmarks;
using Something.Domain.Exceptions;

namespace Something.Infrastructure.Tests;

public sealed class BookmarkRepositoryTests
{
    [Fact]
    public async Task UrlsAreNormalizedValidatedAndDeduplicated()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await BookmarkRepositoryTestHarness.CreateAsync(cancellationToken);

        var bookmark = await harness.Service.CreateAsync(
            new CreateBookmarkRequest(
                " HTTPS://Example.COM:443/path?q=1 ",
                " Example ",
                "  Description  ",
                ["Docs", "docs"]),
            cancellationToken);

        Assert.Equal("https://example.com/path?q=1", bookmark.Url);
        Assert.Equal("Example", bookmark.Title);
        Assert.Equal("Description", bookmark.Description);
        Assert.Equal(["Docs"], bookmark.Tags);

        await Assert.ThrowsAsync<ConflictException>(() => harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://EXAMPLE.com:443/path?q=1", "Duplicate", "", []),
            cancellationToken));
        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.CreateAsync(
            new CreateBookmarkRequest("ftp://example.com/file", "FTP", "", []),
            cancellationToken));
        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://user:secret@example.com", "Credentials", "", []),
            cancellationToken));
    }

    [Fact]
    public async Task SearchStatusAndTagFiltersComposeAndTreatWildcardsLiterally()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await BookmarkRepositoryTestHarness.CreateAsync(cancellationToken);

        var percent = await harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://example.com/coverage", "Reach 100%", "target", ["Dev", "Urgent"]),
            cancellationToken);
        var docs = await harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://docs.example.com", "Manual", "API guide", ["Dev", "Docs"]),
            cancellationToken);
        await harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://news.example.com", "News", "Daily", ["Reading"]),
            cancellationToken);
        docs = await harness.Service.SetReadAsync(
            new SetBookmarkReadRequest(docs.Id, true, docs.Revision), cancellationToken);

        var literal = await harness.Service.ListAsync(
            BookmarkFilter.Default with { Query = "%" }, cancellationToken);
        Assert.Equal(percent.Id, Assert.Single(literal.Bookmarks).Id);

        var tagIntersection = await harness.Service.ListAsync(
            BookmarkFilter.Default with { Tags = ["dev", "urgent"] }, cancellationToken);
        Assert.Equal(percent.Id, Assert.Single(tagIntersection.Bookmarks).Id);

        var read = await harness.Service.ListAsync(
            BookmarkFilter.Default with { Status = BookmarkReadStatus.Read, Query = "api" },
            cancellationToken);
        Assert.Equal(docs.Id, Assert.Single(read.Bookmarks).Id);
        Assert.NotNull(docs.ReadAt);
    }

    [Fact]
    public async Task RevisionsConflictAndReadChangesAreIdempotent()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await BookmarkRepositoryTestHarness.CreateAsync(cancellationToken);

        var created = await harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://example.com", "Example", "", ["One"]),
            cancellationToken);
        var updated = await harness.Service.UpdateAsync(
            new UpdateBookmarkRequest(
                created.Id, created.Url, "Updated", "Body", ["Two"], created.Revision),
            cancellationToken);
        Assert.Equal(2, updated.Revision);

        await Assert.ThrowsAsync<ConflictException>(() => harness.Service.UpdateAsync(
            new UpdateBookmarkRequest(
                created.Id, created.Url, "Stale", "", [], created.Revision),
            cancellationToken));

        var read = await harness.Service.SetReadAsync(
            new SetBookmarkReadRequest(updated.Id, true, updated.Revision), cancellationToken);
        Assert.Equal(3, read.Revision);
        var idempotent = await harness.Service.SetReadAsync(
            new SetBookmarkReadRequest(read.Id, true, created.Revision), cancellationToken);
        Assert.Equal(read.Id, idempotent.Id);
        Assert.Equal(read.Revision, idempotent.Revision);
        Assert.Equal(read.ReadAt, idempotent.ReadAt);
        Assert.Equal(read.Tags, idempotent.Tags);
        await Assert.ThrowsAsync<ConflictException>(() => harness.Service.SetReadAsync(
            new SetBookmarkReadRequest(read.Id, false, created.Revision), cancellationToken));
    }

    [Fact]
    public async Task DeleteRemovesUnusedTagsAndUnknownIdsFail()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await BookmarkRepositoryTestHarness.CreateAsync(cancellationToken);

        var bookmark = await harness.Service.CreateAsync(
            new CreateBookmarkRequest("https://example.com", "Example", "", ["Temporary"]),
            cancellationToken);
        Assert.Equal(["Temporary"], await harness.Service.ListTagsAsync(cancellationToken));

        await harness.Service.DeleteAsync(bookmark.Id, cancellationToken);
        Assert.Empty(await harness.Service.ListTagsAsync(cancellationToken));
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.GetAsync(bookmark.Id, cancellationToken));
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.DeleteAsync(99, cancellationToken));
    }
}
