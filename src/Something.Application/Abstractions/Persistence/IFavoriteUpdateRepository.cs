using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Application.Abstractions.Persistence;

public sealed record FavoriteUpdateSource(
    FavoriteSource Source,
    string SourceId,
    DateTimeOffset AddedAt);

public sealed record FavoriteSourceTracking(
    DateTimeOffset CheckedThrough,
    bool HasSeenItems);

public interface IFavoriteUpdateRepository
{
    Task RecordApplicationOpenAsync(
        DateTimeOffset openedAt,
        CancellationToken cancellationToken = default);

    Task<FavoriteUpdateState> GetStateAsync(CancellationToken cancellationToken = default);

    Task<IReadOnlyList<FavoriteUpdateSource>> ListSourcesAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default);

    Task<FavoriteSourceTracking> GetSourceTrackingAsync(
        FavoriteUpdateSource source,
        DateTimeOffset fallback,
        CancellationToken cancellationToken = default);

    Task<IReadOnlyList<FavoriteUpdateItem>> PersistSuccessfulSourceAsync(
        FavoriteUpdateSource source,
        FavoriteSourceTracking tracking,
        DateTimeOffset defaultCheckedThrough,
        DateTimeOffset scanStartedAt,
        IReadOnlyList<FavoriteUpdateItem> items,
        CancellationToken cancellationToken = default);

    Task RecordSourceFailureAsync(
        FavoriteUpdateSource source,
        DateTimeOffset checkedThrough,
        DateTimeOffset attemptedAt,
        string error,
        CancellationToken cancellationToken = default);

    Task CompleteScanAsync(
        DateTimeOffset scanStartedAt,
        IReadOnlyList<FavoriteUpdateError> errors,
        bool isRefresh,
        CancellationToken cancellationToken = default);

    Task<FavoriteUpdateScanResult> GetCurrentAsync(CancellationToken cancellationToken = default);
}
