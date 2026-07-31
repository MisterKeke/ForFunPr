namespace Something.Application.Features.Notes;

public sealed record SetNoteStateRequest(long Id, bool Value, int ExpectedRevision);
