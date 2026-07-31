using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Application.Abstractions.Persistence;

public interface IFavoriteCategoryRepository
{
    Task<IReadOnlyList<FavoriteCategory>> ListAsync(FavoriteSource source, CancellationToken cancellationToken = default);
    Task<(FavoriteCategory Category, bool Created)> CreateAsync(
        string name,
        FavoriteSource source,
        CancellationToken cancellationToken = default);
    Task<FavoriteCategory> RenameAsync(long id, string name, CancellationToken cancellationToken = default);
    Task<bool> ExistsAsync(long id, FavoriteSource source, CancellationToken cancellationToken = default);
}
