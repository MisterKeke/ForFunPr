using Something.Domain.Enums;

namespace Something.Domain.Models.Favorites;

public sealed record FavoriteUpdateError(
    FavoriteSource Source,
    string SourceId,
    string Message);
