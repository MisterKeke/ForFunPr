namespace Something.Domain.Models.Tasks;

public sealed record TaskSubtaskInput(
    long Id,
    string Title,
    bool IsCompleted,
    int Position = 0);
