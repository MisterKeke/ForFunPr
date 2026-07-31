namespace Something.Domain.Models.Favorites;

public sealed record FavoriteUpdateScanResult(
    DateTimeOffset? ScanStartedAt,
    IReadOnlyList<FavoriteUpdateItem> Updates,
    IReadOnlyList<FavoriteUpdateItem> NewUpdates,
    IReadOnlyList<FavoriteUpdateError> Errors,
    FavoriteUpdateState State);
