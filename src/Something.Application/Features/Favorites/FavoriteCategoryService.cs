using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Messages;
using Something.Domain.Enums;
using Something.Domain.Models.Favorites;
using Something.Domain.Validation;

namespace Something.Application.Features.Favorites;

public sealed class FavoriteCategoryService(
    IFavoriteCategoryRepository repository,
    IMessenger messenger) : IFavoriteCategoryService
{
    public Task<IReadOnlyList<FavoriteCategory>> ListAsync(
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        _ = FavoriteRules.ToDatabase(source);
        return repository.ListAsync(source, cancellationToken);
    }

    public async Task<FavoriteCategory> CreateAsync(
        string name,
        FavoriteSource source,
        CancellationToken cancellationToken = default)
    {
        name = FavoriteRules.NormalizeCategoryName(name);
        _ = FavoriteRules.ToDatabase(source);
        var (category, created) = await repository.CreateAsync(name, source, cancellationToken);
        if (created)
        {
            messenger.Send(new FavoriteCategoriesChangedMessage(source, DateTimeOffset.UtcNow));
        }

        return category;
    }

    public async Task<FavoriteCategory> RenameAsync(
        long id,
        string name,
        CancellationToken cancellationToken = default)
    {
        FavoriteRules.RequirePositiveCategoryId(id);
        name = FavoriteRules.NormalizeCategoryName(name);
        var category = await repository.RenameAsync(id, name, cancellationToken);
        messenger.Send(new FavoriteCategoriesChangedMessage(category.Source, DateTimeOffset.UtcNow));
        return category;
    }
}
