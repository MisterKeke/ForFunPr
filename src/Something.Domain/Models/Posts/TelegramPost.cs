namespace Something.Domain.Models.Posts;

public sealed record TelegramPost(
    string Text,
    IReadOnlyList<string> ImageUrls,
    DateTimeOffset? PublishedAt,
    string Views,
    string PostId)
{
    public string PublishedLabel => PublishedAt?.LocalDateTime.ToString("g") ?? string.Empty;
    public string ImageSummary => ImageUrls.Count == 0
        ? string.Empty
        : ImageUrls.Count == 1 ? "1 image" : $"{ImageUrls.Count} images";
}
