using Something.Domain.Enums;

namespace Something.Domain.Models.Favorites;

public sealed record FavoriteChannel(
    string SourceId,
    string DisplayName,
    FavoriteSource Source,
    long? CategoryId);
