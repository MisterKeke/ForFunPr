using Something.Application.Features.Notes;
using Something.Domain.Exceptions;

namespace Something.Infrastructure.Tests;

public sealed class NoteRepositoryTests
{
    [Fact]
    public async Task CreateListAndGetPreserveNormalizedContent()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await NoteRepositoryTestHarness.CreateAsync(cancellationToken);

        var body = "First line\n\nSecond   line";
        var created = await harness.Service.CreateAsync(
            new CreateNoteRequest("  Release notes  ", body, true),
            cancellationToken);

        Assert.Equal("Release notes", created.Title);
        Assert.Equal(body, created.Body);
        Assert.True(created.IsPinned);
        Assert.Equal(1, created.Revision);

        var result = await harness.Service.ListAsync(cancellationToken: cancellationToken);
        var summary = Assert.Single(result.Notes);
        Assert.Equal("First line Second line", summary.Preview);
        Assert.Equal(created, await harness.Service.GetAsync(created.Id, cancellationToken));
    }

    [Fact]
    public async Task ListSupportsArchivePinnedPagingAndLiteralWildcardSearch()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await NoteRepositoryTestHarness.CreateAsync(cancellationToken);

        var percent = await harness.Service.CreateAsync(
            new CreateNoteRequest("Reach 100%", "Coverage target", true), cancellationToken);
        var archived = await harness.Service.CreateAsync(
            new CreateNoteRequest("Old draft", "Archived material"), cancellationToken);
        await harness.Service.CreateAsync(
            new CreateNoteRequest("Current draft", "Active material"), cancellationToken);
        archived = await harness.Service.SetArchivedAsync(
            new SetNoteStateRequest(archived.Id, true, archived.Revision), cancellationToken);

        var active = await harness.Service.ListAsync(
            NoteListFilter.Default with { Limit = 1 }, cancellationToken);
        Assert.Equal(2, active.Total);
        Assert.Single(active.Notes);
        Assert.Equal(percent.Id, active.Notes[0].Id);

        var secondPage = await harness.Service.ListAsync(
            NoteListFilter.Default with { Limit = 1, Offset = 1 }, cancellationToken);
        Assert.Single(secondPage.Notes);
        Assert.NotEqual(percent.Id, secondPage.Notes[0].Id);

        var archivedOnly = await harness.Service.ListAsync(
            NoteListFilter.Default with { ArchiveStatus = NoteArchiveStatus.Archived },
            cancellationToken);
        Assert.Equal(archived.Id, Assert.Single(archivedOnly.Notes).Id);

        var pinnedOnly = await harness.Service.ListAsync(
            NoteListFilter.Default with { ArchiveStatus = NoteArchiveStatus.All, IsPinned = true },
            cancellationToken);
        Assert.Equal(percent.Id, Assert.Single(pinnedOnly.Notes).Id);

        var literalWildcard = await harness.Service.ListAsync(
            NoteListFilter.Default with { ArchiveStatus = NoteArchiveStatus.All, Query = "%" },
            cancellationToken);
        Assert.Equal(percent.Id, Assert.Single(literalWildcard.Notes).Id);
    }

    [Fact]
    public async Task RevisionConflictsArePreservedAndStateChangesAreIdempotent()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await NoteRepositoryTestHarness.CreateAsync(cancellationToken);

        var created = await harness.Service.CreateAsync(
            new CreateNoteRequest("Draft", "Version one"), cancellationToken);
        var updated = await harness.Service.UpdateAsync(
            new UpdateNoteRequest(created.Id, "Draft", "Version two", created.Revision),
            cancellationToken);
        Assert.Equal(2, updated.Revision);

        await Assert.ThrowsAsync<ConflictException>(() => harness.Service.UpdateAsync(
            new UpdateNoteRequest(created.Id, "Stale", "write", created.Revision),
            cancellationToken));

        var pinned = await harness.Service.SetPinnedAsync(
            new SetNoteStateRequest(updated.Id, true, updated.Revision), cancellationToken);
        Assert.Equal(3, pinned.Revision);

        var idempotent = await harness.Service.SetPinnedAsync(
            new SetNoteStateRequest(pinned.Id, true, created.Revision), cancellationToken);
        Assert.Equal(pinned, idempotent);

        await Assert.ThrowsAsync<ConflictException>(() => harness.Service.SetArchivedAsync(
            new SetNoteStateRequest(pinned.Id, true, created.Revision), cancellationToken));
    }

    [Fact]
    public async Task RestoreDeleteAndValidationBehaveConsistently()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await NoteRepositoryTestHarness.CreateAsync(cancellationToken);

        var note = await harness.Service.CreateAsync(
            new CreateNoteRequest(string.Empty, "Body-only note"), cancellationToken);
        Assert.Equal("Body-only note", note.DisplayTitle);

        note = await harness.Service.SetArchivedAsync(
            new SetNoteStateRequest(note.Id, true, note.Revision), cancellationToken);
        Assert.True(note.IsArchived);
        note = await harness.Service.SetArchivedAsync(
            new SetNoteStateRequest(note.Id, false, note.Revision), cancellationToken);
        Assert.False(note.IsArchived);

        await harness.Service.DeleteAsync(note.Id, cancellationToken);
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Service.GetAsync(note.Id, cancellationToken));
        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.CreateAsync(
            new CreateNoteRequest("", "   "), cancellationToken));
    }
}
