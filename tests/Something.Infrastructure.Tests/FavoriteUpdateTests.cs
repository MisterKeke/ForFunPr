using Something.Application.Abstractions.Providers;
using Something.Application.Features.Favorites;
using Something.Domain.Enums;
using Something.Domain.Models.Posts;

namespace Something.Infrastructure.Tests;

public sealed class FavoriteUpdateTests
{
    [Fact]
    public async Task InitialScanBaselinesHistoryAndRefreshPersistsOnlyNewItems()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await FavoriteRepositoryTestHarness.CreateAsync(cancellationToken);
        await harness.Channels.AddTelegramAsync("example", cancellationToken);

        var clock = new MutableTimeProvider(new DateTimeOffset(2030, 1, 1, 10, 0, 0, TimeSpan.Zero));
        var telegram = new StubTelegramProvider
        {
            Posts = [Post("1", clock.GetUtcNow().AddMinutes(-5))],
        };
        var service = new FavoriteUpdateService(
            harness.Updates,
            telegram,
            new StubYouTubeProvider(),
            clock);

        await service.RecordApplicationOpenAsync(cancellationToken);
        clock.Advance(TimeSpan.FromMinutes(1));
        var initial = await service.GetInitialAsync(cancellationToken);
        Assert.Empty(initial.Updates);
        Assert.Empty(initial.NewUpdates);

        telegram.Posts =
        [
            Post("2", clock.GetUtcNow().AddSeconds(30)),
            Post("1", clock.GetUtcNow().AddMinutes(-6)),
        ];
        clock.Advance(TimeSpan.FromMinutes(1));
        var refresh = await service.RefreshAsync(cancellationToken);
        var update = Assert.Single(refresh.Updates);
        Assert.Equal("2", update.ItemId);
        Assert.Single(refresh.NewUpdates);
        Assert.Equal(clock.GetUtcNow(), refresh.State.LastRefreshAt);

        clock.Advance(TimeSpan.FromMinutes(1));
        var repeated = await service.RefreshAsync(cancellationToken);
        Assert.Empty(repeated.Updates);
        Assert.Empty(repeated.NewUpdates);
    }

    [Fact]
    public async Task ProviderFailureIsPartialAndDoesNotAdvanceFailedCheckpoint()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await FavoriteRepositoryTestHarness.CreateAsync(cancellationToken);
        await harness.Channels.AddTelegramAsync("example", cancellationToken);
        const string channelId = "UCabcdefghijklmnopqrstuv";
        await harness.Channels.AddYouTubeAsync(channelId, "Example", cancellationToken);

        var clock = new MutableTimeProvider(new DateTimeOffset(2030, 2, 1, 8, 0, 0, TimeSpan.Zero));
        var telegram = new StubTelegramProvider { Posts = [] };
        var youtube = new StubYouTubeProvider { Failure = new HttpRequestException("offline") };
        var service = new FavoriteUpdateService(harness.Updates, telegram, youtube, clock);
        await service.RecordApplicationOpenAsync(cancellationToken);

        clock.Advance(TimeSpan.FromMinutes(1));
        var partial = await service.RefreshAsync(cancellationToken);
        var error = Assert.Single(partial.Errors);
        Assert.Equal(FavoriteSource.YouTube, error.Source);
        Assert.Equal(channelId, error.SourceId);
        var source = Assert.Single(await harness.Updates.ListSourcesAsync(FavoriteSource.YouTube, cancellationToken));
        var failedTracking = await harness.Updates.GetSourceTrackingAsync(
            source,
            partial.State.CurrentOpenedAt!.Value,
            cancellationToken);
        Assert.Equal(partial.State.CurrentOpenedAt, failedTracking.CheckedThrough);

        youtube.Failure = null;
        youtube.Videos =
        [
            new YouTubeVideo(
                "abcdefghijk", "Recovered", "", "", clock.GetUtcNow().AddSeconds(30),
                channelId, "Example", "https://www.youtube.com/watch?v=abcdefghijk", "", ""),
        ];
        clock.Advance(TimeSpan.FromMinutes(1));
        var recovered = await service.RefreshAsync(cancellationToken);
        Assert.Empty(recovered.Errors);
        Assert.Equal("abcdefghijk", Assert.Single(recovered.Updates).ItemId);
    }

    private static TelegramPost Post(string id, DateTimeOffset publishedAt) =>
        new($"Post {id}", [], publishedAt, "", id);

    private sealed class MutableTimeProvider(DateTimeOffset value) : TimeProvider
    {
        private DateTimeOffset _value = value;
        public override DateTimeOffset GetUtcNow() => _value;
        public void Advance(TimeSpan amount) => _value += amount;
    }

    private sealed class StubTelegramProvider : ITelegramProvider
    {
        public IReadOnlyList<TelegramPost> Posts { get; set; } = [];
        public Exception? Failure { get; set; }

        public Task<IReadOnlyList<TelegramPost>> GetPostsAsync(
            string username,
            int before = 0,
            bool forceRefresh = false,
            CancellationToken cancellationToken = default)
        {
            cancellationToken.ThrowIfCancellationRequested();
            return Failure is null
                ? Task.FromResult(Posts)
                : Task.FromException<IReadOnlyList<TelegramPost>>(Failure);
        }
    }

    private sealed class StubYouTubeProvider : IYouTubeProvider
    {
        public IReadOnlyList<YouTubeVideo> Videos { get; set; } = [];
        public Exception? Failure { get; set; }

        public Task<(string ChannelId, string Handle)> ResolveChannelAsync(
            string reference,
            CancellationToken cancellationToken = default) =>
            Task.FromResult((reference, reference));

        public Task<IReadOnlyList<YouTubeVideo>> GetVideosAsync(
            string reference,
            bool forceRefresh = false,
            CancellationToken cancellationToken = default)
        {
            cancellationToken.ThrowIfCancellationRequested();
            return Failure is null
                ? Task.FromResult(Videos)
                : Task.FromException<IReadOnlyList<YouTubeVideo>>(Failure);
        }
    }
}
