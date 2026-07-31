using Something.Application.Abstractions.Persistence;
using Something.Application.Abstractions.Providers;
using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Application.Features.Favorites;

public sealed class FavoriteUpdateService(
    IFavoriteUpdateRepository repository,
    ITelegramProvider telegramProvider,
    IYouTubeProvider youTubeProvider,
    TimeProvider timeProvider) : IFavoriteUpdateService
{
    private readonly SemaphoreSlim _scanLock = new(1, 1);

    public Task RecordApplicationOpenAsync(CancellationToken cancellationToken = default) =>
        repository.RecordApplicationOpenAsync(timeProvider.GetUtcNow(), cancellationToken);

    public Task<FavoriteUpdateState> GetStateAsync(CancellationToken cancellationToken = default) =>
        repository.GetStateAsync(cancellationToken);

    public Task<FavoriteUpdateScanResult> GetCurrentAsync(CancellationToken cancellationToken = default) =>
        repository.GetCurrentAsync(cancellationToken);

    public Task<FavoriteUpdateScanResult> GetInitialAsync(CancellationToken cancellationToken = default) =>
        ScanAsync(isRefresh: false, cancellationToken);

    public Task<FavoriteUpdateScanResult> RefreshAsync(CancellationToken cancellationToken = default) =>
        ScanAsync(isRefresh: true, cancellationToken);

    private async Task<FavoriteUpdateScanResult> ScanAsync(
        bool isRefresh,
        CancellationToken cancellationToken)
    {
        await _scanLock.WaitAsync(cancellationToken);
        try
        {
            var scanStartedAt = timeProvider.GetUtcNow();
            var state = await repository.GetStateAsync(cancellationToken);
            var defaultCheckedThrough = isRefresh
                ? state.LastRefreshAt ?? state.CurrentOpenedAt ?? scanStartedAt
                : state.PreviousOpenedAt ?? state.CurrentOpenedAt ?? scanStartedAt;
            var newItems = new List<FavoriteUpdateItem>();
            var errors = new List<FavoriteUpdateError>();

            await ScanSourceAsync(FavoriteSource.Telegram, defaultCheckedThrough, scanStartedAt, newItems, errors, cancellationToken);
            await ScanSourceAsync(FavoriteSource.YouTube, defaultCheckedThrough, scanStartedAt, newItems, errors, cancellationToken);

            await repository.CompleteScanAsync(scanStartedAt, errors, isRefresh, cancellationToken);
            var current = await repository.GetCurrentAsync(cancellationToken);
            return current with
            {
                NewUpdates = newItems.OrderByDescending(item => item.PublishedAt).ToArray(),
            };
        }
        finally
        {
            _scanLock.Release();
        }
    }

    private async Task ScanSourceAsync(
        FavoriteSource source,
        DateTimeOffset defaultCheckedThrough,
        DateTimeOffset scanStartedAt,
        List<FavoriteUpdateItem> newItems,
        List<FavoriteUpdateError> errors,
        CancellationToken cancellationToken)
    {
        IReadOnlyList<FavoriteUpdateSource> sources;
        try
        {
            sources = await repository.ListSourcesAsync(source, cancellationToken);
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            _ = exception;
            errors.Add(new FavoriteUpdateError(source, string.Empty, $"{SourceLabel(source)} favorites could not be read."));
            return;
        }

        foreach (var favorite in sources)
        {
            cancellationToken.ThrowIfCancellationRequested();
            FavoriteSourceTracking tracking;
            try
            {
                tracking = await repository.GetSourceTrackingAsync(favorite, defaultCheckedThrough, cancellationToken);
            }
            catch (Exception exception) when (exception is not OperationCanceledException)
            {
                _ = exception;
                errors.Add(new FavoriteUpdateError(source, favorite.SourceId, $"The {SourceLabel(source)} refresh checkpoint could not be read."));
                continue;
            }

            try
            {
                var items = source == FavoriteSource.Telegram
                    ? await LoadTelegramAsync(favorite.SourceId, tracking.CheckedThrough, cancellationToken)
                    : await LoadYouTubeAsync(favorite.SourceId, tracking.CheckedThrough, cancellationToken);
                var additions = await repository.PersistSuccessfulSourceAsync(
                    favorite,
                    tracking,
                    defaultCheckedThrough,
                    scanStartedAt,
                    items,
                    cancellationToken);
                newItems.AddRange(additions);
            }
            catch (Exception exception) when (exception is not OperationCanceledException)
            {
                errors.Add(new FavoriteUpdateError(
                    source,
                    favorite.SourceId,
                    $"{SourceLabel(source)} data could not be loaded."));
                try
                {
                    await repository.RecordSourceFailureAsync(
                        favorite,
                        tracking.CheckedThrough,
                        scanStartedAt,
                        exception.Message,
                        cancellationToken);
                }
                catch (Exception persistenceException) when (persistenceException is not OperationCanceledException)
                {
                    _ = persistenceException;
                    errors.Add(new FavoriteUpdateError(
                        source,
                        favorite.SourceId,
                        $"The {SourceLabel(source)} failure checkpoint could not be saved."));
                }
            }
        }
    }

    private async Task<IReadOnlyList<FavoriteUpdateItem>> LoadTelegramAsync(
        string username,
        DateTimeOffset checkedThrough,
        CancellationToken cancellationToken)
    {
        var posts = await telegramProvider.GetPostsAsync(username, 0, false, cancellationToken);
        return posts
            .Where(post => post.PublishedAt is not null && !string.IsNullOrWhiteSpace(post.PostId))
            .Select(post => new FavoriteUpdateItem(
                FavoriteSource.Telegram,
                checkedThrough,
                post.PublishedAt!.Value,
                username,
                post.PostId,
                $"@{username}",
                post.Text,
                post.ImageUrls,
                post.Views,
                BuildTelegramUrl(username, post.PostId),
                string.Empty,
                string.Empty,
                string.Empty))
            .ToArray();
    }

    private async Task<IReadOnlyList<FavoriteUpdateItem>> LoadYouTubeAsync(
        string channelId,
        DateTimeOffset checkedThrough,
        CancellationToken cancellationToken)
    {
        var videos = await youTubeProvider.GetVideosAsync(channelId, false, cancellationToken);
        return videos
            .Where(video => video.PublishedAt is not null && !string.IsNullOrWhiteSpace(video.VideoId))
            .Select(video => new FavoriteUpdateItem(
                FavoriteSource.YouTube,
                checkedThrough,
                video.PublishedAt!.Value,
                channelId,
                video.VideoId,
                video.Title,
                video.ChannelTitle,
                [],
                video.Views,
                video.VideoUrl,
                video.ThumbnailUrl,
                video.Description,
                video.Duration))
            .ToArray();
    }

    private static string BuildTelegramUrl(string username, string postId)
    {
        username = username.Trim().TrimStart('@');
        postId = postId.Trim().Trim('/');
        if (postId.Contains('/'))
        {
            var parts = postId.Split('/', StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length == 2)
            {
                username = parts[0].TrimStart('@');
                postId = parts[1];
            }
        }

        return Uri.TryCreate($"https://t.me/{username}/{postId}", UriKind.Absolute, out var uri)
            ? uri.AbsoluteUri
            : string.Empty;
    }

    private static string SourceLabel(FavoriteSource source) =>
        source == FavoriteSource.Telegram ? "Telegram" : "YouTube";
}
