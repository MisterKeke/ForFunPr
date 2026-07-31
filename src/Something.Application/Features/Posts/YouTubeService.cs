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

public sealed class YouTubeService(
    IYouTubeProvider provider,
    IFavoriteChannelRepository favorites,
    IFavoriteCategoryRepository categories,
    IMessenger messenger) : IYouTubeService
{
    public Task<IReadOnlyList<YouTubeVideo>> GetVideosAsync(
        string channel,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default)
    {
        _ = ChannelRules.NormalizeYouTubeReference(channel);
        return provider.GetVideosAsync(channel, forceRefresh, cancellationToken);
    }

    public Task<IReadOnlyList<YouTubeVideo>> GetVideosPaginatedAsync(
        string channel,
        int before,
        CancellationToken cancellationToken = default)
    {
        _ = channel;
        _ = before;
        cancellationToken.ThrowIfCancellationRequested();
        throw new NotSupportedException("YouTube RSS does not support pagination.");
    }

    public Task<IReadOnlyList<FavoriteChannel>> ListFavoritesAsync(CancellationToken cancellationToken = default)
    {
        return favorites.ListAsync(FavoriteSource.YouTube, cancellationToken);
    }

    public async Task AddFavoriteAsync(
        string channel,
        long categoryId,
        CancellationToken cancellationToken = default)
    {
        FavoriteRules.RequirePositiveCategoryId(categoryId);
        if (!await categories.ExistsAsync(categoryId, FavoriteSource.YouTube, cancellationToken))
        {
            throw new NotFoundException($"Favorite category {categoryId} was not found.");
        }

        var (channelId, handle) = await provider.ResolveChannelAsync(channel, cancellationToken);
        await favorites.AddYouTubeAsync(channelId, handle, cancellationToken);
        await favorites.AssignCategoryAsync(FavoriteSource.YouTube, channelId, categoryId, cancellationToken);
        PublishChanged();
    }

    public async Task RemoveFavoriteAsync(string channel, CancellationToken cancellationToken = default)
    {
        var (channelId, _) = await provider.ResolveChannelAsync(channel, cancellationToken);
        await favorites.RemoveAsync(FavoriteSource.YouTube, channelId, cancellationToken);
        PublishChanged();
    }

    private void PublishChanged() =>
        messenger.Send(new FavoritesChangedMessage("youtube", DateTimeOffset.UtcNow));
}
