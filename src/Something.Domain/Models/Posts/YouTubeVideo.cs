namespace Something.Domain.Models.Posts;

public sealed record YouTubeVideo(
    string VideoId,
    string Title,
    string Description,
    string ThumbnailUrl,
    DateTimeOffset? PublishedAt,
    string ChannelId,
    string ChannelTitle,
    string VideoUrl,
    string Views,
    string Duration)
{
    public string PublishedLabel => PublishedAt?.LocalDateTime.ToString("g") ?? string.Empty;
}
