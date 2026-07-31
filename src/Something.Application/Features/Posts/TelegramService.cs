using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Abstractions.Providers;
using Something.Application.Messages;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Favorites;
using Something.Domain.Models.Posts;
using Something.Domain.Validation;

namespace Something.Application.Features.Posts;

public sealed class TelegramService(
    ITelegramProvider provider,
    IFavoriteChannelRepository favorites,
    IFavoriteCategoryRepository categories,
    IMessenger messenger) : ITelegramService
{
    public Task<IReadOnlyList<TelegramPost>> GetPostsAsync(
        string username,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default)
    {
        username = ChannelRules.NormalizeTelegramUsername(username);
        return provider.GetPostsAsync(username, 0, forceRefresh, cancellationToken);
    }

    public Task<IReadOnlyList<TelegramPost>> GetOlderPostsAsync(
        string username,
        int before,
        CancellationToken cancellationToken = default)
    {
        username = ChannelRules.NormalizeTelegramUsername(username);
        if (before < 0)
        {
            throw new ValidationException("Telegram pagination cursor cannot be negative.");
        }

        return provider.GetPostsAsync(username, before, true, cancellationToken);
    }

    public Task<IReadOnlyList<FavoriteChannel>> ListFavoritesAsync(CancellationToken cancellationToken = default)
    {
        return favorites.ListAsync(FavoriteSource.Telegram, cancellationToken);
    }

    public async Task AddFavoriteAsync(
        string username,
        long categoryId,
        CancellationToken cancellationToken = default)
    {
        username = ChannelRules.NormalizeTelegramUsername(username);
        FavoriteRules.RequirePositiveCategoryId(categoryId);
        if (!await categories.ExistsAsync(categoryId, FavoriteSource.Telegram, cancellationToken))
        {
            throw new NotFoundException($"Favorite category {categoryId} was not found.");
        }

        await favorites.AddTelegramAsync(username, cancellationToken);
        await favorites.AssignCategoryAsync(FavoriteSource.Telegram, username, categoryId, cancellationToken);
        PublishChanged();
    }

    public async Task RemoveFavoriteAsync(string username, CancellationToken cancellationToken = default)
    {
        username = ChannelRules.NormalizeTelegramUsername(username);
        await favorites.RemoveAsync(FavoriteSource.Telegram, username, cancellationToken);
        PublishChanged();
    }

    private void PublishChanged() =>
        messenger.Send(new FavoritesChangedMessage("telegram", DateTimeOffset.UtcNow));
}
