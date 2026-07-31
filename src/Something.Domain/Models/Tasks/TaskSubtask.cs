namespace Something.Domain.Models.Tasks;

public sealed record TaskSubtask(
    long Id,
    string Title,
    bool IsCompleted,
    int Position);
