using Something.Domain.Enums;

namespace Something.Domain.Models.Favorites;

public sealed record FavoriteUpdateItem(
    FavoriteSource Source,
    DateTimeOffset CheckedThrough,
    DateTimeOffset PublishedAt,
    string SourceId,
    string ItemId,
    string Title,
    string Preview,
    IReadOnlyList<string> ImageUrls,
    string Views,
    string Url,
    string ThumbnailUrl,
    string Description,
    string Duration)
{
    public string SourceLabel => Source == FavoriteSource.Telegram ? "Telegram" : "YouTube";
    public string DisplayTitle => string.IsNullOrWhiteSpace(Title) ? SourceId : Title;
    public string PublishedLabel => PublishedAt.LocalDateTime.ToString("g");
}
