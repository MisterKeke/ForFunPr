namespace Something.Application.Features.Notes;

public sealed record UpdateNoteRequest(
    long Id,
    string Title,
    string Body,
    int ExpectedRevision);
