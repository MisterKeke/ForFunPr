using Something.Domain.Models.Favorites;

namespace Something.Application.Features.Favorites;

public interface IFavoriteUpdateService
{
    Task RecordApplicationOpenAsync(CancellationToken cancellationToken = default);
    Task<FavoriteUpdateState> GetStateAsync(CancellationToken cancellationToken = default);
    Task<FavoriteUpdateScanResult> GetCurrentAsync(CancellationToken cancellationToken = default);
    Task<FavoriteUpdateScanResult> GetInitialAsync(CancellationToken cancellationToken = default);
    Task<FavoriteUpdateScanResult> RefreshAsync(CancellationToken cancellationToken = default);
}
