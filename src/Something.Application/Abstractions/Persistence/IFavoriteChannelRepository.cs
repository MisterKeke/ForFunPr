using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Application.Abstractions.Persistence;

public interface IFavoriteChannelRepository
{
    Task AddTelegramAsync(string username, CancellationToken cancellationToken = default);
    Task AddYouTubeAsync(string channelId, string handle, CancellationToken cancellationToken = default);
    Task RemoveAsync(FavoriteSource source, string sourceId, CancellationToken cancellationToken = default);
    Task AssignCategoryAsync(
        FavoriteSource source,
        string sourceId,
        long categoryId,
        CancellationToken cancellationToken = default);
    Task<IReadOnlyList<FavoriteChannel>> ListAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default);
}
