using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Application.Features.Favorites;

public interface IFavoriteCategoryService
{
    Task<IReadOnlyList<FavoriteCategory>> ListAsync(FavoriteSource source, CancellationToken cancellationToken = default);
    Task<FavoriteCategory> CreateAsync(string name, FavoriteSource source, CancellationToken cancellationToken = default);
    Task<FavoriteCategory> RenameAsync(long id, string name, CancellationToken cancellationToken = default);
}
