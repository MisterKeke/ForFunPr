using Something.Domain.Models.Favorites;
using Something.Domain.Models.Posts;

namespace Something.Application.Features.Posts;

public interface ITelegramService
{
    Task<IReadOnlyList<TelegramPost>> GetPostsAsync(string username, bool forceRefresh = false, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<TelegramPost>> GetOlderPostsAsync(string username, int before, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<FavoriteChannel>> ListFavoritesAsync(CancellationToken cancellationToken = default);
    Task AddFavoriteAsync(string username, long categoryId, CancellationToken cancellationToken = default);
    Task RemoveFavoriteAsync(string username, CancellationToken cancellationToken = default);
}
