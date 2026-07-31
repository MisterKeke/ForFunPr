using Something.Domain.Enums;

namespace Something.Domain.Models.Favorites;

public sealed record FavoriteCategory(
    long Id,
    string Name,
    FavoriteSource Source,
    string Color,
    DateTimeOffset CreatedAt);
