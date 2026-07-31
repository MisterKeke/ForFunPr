namespace Something.Application.Features.Notes;

public sealed record CreateNoteRequest(string Title, string Body, bool IsPinned = false);
