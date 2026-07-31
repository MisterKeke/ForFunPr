using Something.Domain.Models.Favorites;
using Something.Domain.Models.Posts;

namespace Something.Application.Features.Posts;

public interface IYouTubeService
{
    Task<IReadOnlyList<YouTubeVideo>> GetVideosAsync(string channel, bool forceRefresh = false, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<YouTubeVideo>> GetVideosPaginatedAsync(string channel, int before, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<FavoriteChannel>> ListFavoritesAsync(CancellationToken cancellationToken = default);
    Task AddFavoriteAsync(string channel, long categoryId, CancellationToken cancellationToken = default);
    Task RemoveFavoriteAsync(string channel, CancellationToken cancellationToken = default);
}
