using System.Net;
using System.Text;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Infrastructure.Providers.Telegram;
using Something.Infrastructure.Providers.YouTube;

namespace Something.Infrastructure.Tests;

public sealed class FavoriteAndPostProviderTests
{
    [Fact]
    public async Task CategoriesAreSourceScopedAndRenameConflictsArePreserved()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await FavoriteRepositoryTestHarness.CreateAsync(cancellationToken);

        var telegram = await harness.Categories.CreateAsync(" News ", FavoriteSource.Telegram, cancellationToken);
        var existing = await harness.Categories.CreateAsync("news", FavoriteSource.Telegram, cancellationToken);
        var youtube = await harness.Categories.CreateAsync("News", FavoriteSource.YouTube, cancellationToken);
        Assert.Equal(telegram.Id, existing.Id);
        Assert.NotEqual(telegram.Id, youtube.Id);

        var second = await harness.Categories.CreateAsync("Work", FavoriteSource.Telegram, cancellationToken);
        await Assert.ThrowsAsync<ConflictException>(() =>
            harness.Categories.RenameAsync(second.Id, "NEWS", cancellationToken));
        var renamed = await harness.Categories.RenameAsync(second.Id, "Projects", cancellationToken);
        Assert.Equal("Projects", renamed.Name);
    }

    [Fact]
    public async Task FavoriteAssignmentsRequireMatchingSourceCategories()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await FavoriteRepositoryTestHarness.CreateAsync(cancellationToken);
        var telegramCategory = await harness.Categories.CreateAsync("Telegram", FavoriteSource.Telegram, cancellationToken);
        var youtubeCategory = await harness.Categories.CreateAsync("YouTube", FavoriteSource.YouTube, cancellationToken);

        await harness.Channels.AddTelegramAsync("valid_channel", cancellationToken);
        await harness.Channels.AssignCategoryAsync(
            FavoriteSource.Telegram, "valid_channel", telegramCategory.Id, cancellationToken);
        var telegram = Assert.Single(await harness.Channels.ListAsync(FavoriteSource.Telegram, cancellationToken));
        Assert.Equal(telegramCategory.Id, telegram.CategoryId);
        await Assert.ThrowsAsync<NotFoundException>(() => harness.Channels.AssignCategoryAsync(
            FavoriteSource.Telegram, "valid_channel", youtubeCategory.Id, cancellationToken));

        const string channelId = "UCabcdefghijklmnopqrstuv";
        await harness.Channels.AddYouTubeAsync(channelId, "Example.Handle", cancellationToken);
        await harness.Channels.AssignCategoryAsync(
            FavoriteSource.YouTube, channelId, youtubeCategory.Id, cancellationToken);
        var youtube = Assert.Single(await harness.Channels.ListAsync(FavoriteSource.YouTube, cancellationToken));
        Assert.Equal("Example.Handle", youtube.DisplayName);
    }

    [Fact]
    public async Task TelegramProviderParsesPaginatesAndCachesPosts()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var requestCount = 0;
        const string html = """
            <div class="tgme_widget_message_wrap" data-post="valid_channel/10">
              <div class="tgme_widget_message_text">Older post</div>
              <a class="tgme_widget_message_date" href="https://t.me/valid_channel/10"><time datetime="2026-07-30T10:00:00Z"></time></a>
              <span class="tgme_widget_message_views">100</span>
            </div>
            <div class="tgme_widget_message_wrap" data-post="valid_channel/11">
              <div class="tgme_widget_message_text">Newest post</div>
              <a class="tgme_widget_message_date" href="https://t.me/valid_channel/11"><time datetime="2026-07-31T10:00:00Z"></time></a>
              <a class="tgme_widget_message_photo_wrap" style="background-image:url('https://cdn.example/image.jpg')"></a>
            </div>
            """;
        var handler = new DelegateHttpHandler((request, _) =>
        {
            requestCount++;
            if (request.RequestUri!.Query.Length > 0)
            {
                Assert.Contains("before=10", request.RequestUri.Query, StringComparison.Ordinal);
            }
            return Task.FromResult(HtmlResponse(html));
        });
        using var client = new HttpClient(handler);
        var provider = new TelegramProvider(new StubHttpClientFactory(client));

        var first = await provider.GetPostsAsync("@VALID_CHANNEL", cancellationToken: cancellationToken);
        Assert.Equal("Newest post", first[0].Text);
        Assert.Single(first[0].ImageUrls);
        await provider.GetPostsAsync("valid_channel", cancellationToken: cancellationToken);
        Assert.Equal(1, requestCount);
        await provider.GetPostsAsync("valid_channel", 10, true, cancellationToken);
        Assert.Equal(2, requestCount);
        await provider.GetPostsAsync("valid_channel", forceRefresh: true, cancellationToken: cancellationToken);
        Assert.Equal(3, requestCount);
    }

    [Fact]
    public async Task YouTubeProviderResolvesHandlesParsesRssAndCachesBoth()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        const string channelId = "UCabcdefghijklmnopqrstuv";
        var handleRequests = 0;
        var feedRequests = 0;
        var handler = new DelegateHttpHandler((request, _) =>
        {
            if (request.RequestUri!.AbsolutePath.StartsWith("/@", StringComparison.Ordinal))
            {
                handleRequests++;
                return Task.FromResult(HtmlResponse($"<meta itemprop=\"channelId\" content=\"{channelId}\">"));
            }

            feedRequests++;
            const string feed = """
                <feed xmlns="http://www.w3.org/2005/Atom"
                      xmlns:yt="http://www.youtube.com/xml/schemas/2015"
                      xmlns:media="http://search.yahoo.com/mrss/">
                  <entry>
                    <yt:videoId>abcdefghijk</yt:videoId>
                    <title>Example video</title>
                    <published>2026-07-31T10:00:00Z</published>
                    <author><name>Example channel</name></author>
                    <media:group>
                      <media:thumbnail url="https://img.example/thumb.jpg" />
                      <media:description>Description</media:description>
                      <media:community><media:statistics views="123" /></media:community>
                    </media:group>
                  </entry>
                </feed>
                """;
            return Task.FromResult(XmlResponse(feed));
        });
        using var client = new HttpClient(handler);
        var provider = new YouTubeProvider(new StubHttpClientFactory(client));

        var videos = await provider.GetVideosAsync("@Example.Handle", cancellationToken: cancellationToken);
        var video = Assert.Single(videos);
        Assert.Equal("Example video", video.Title);
        Assert.Equal("123", video.Views);
        Assert.Equal($"https://www.youtube.com/watch?v={video.VideoId}", video.VideoUrl);
        await provider.GetVideosAsync("Example.Handle", cancellationToken: cancellationToken);
        Assert.Equal(1, handleRequests);
        Assert.Equal(1, feedRequests);
        await provider.GetVideosAsync("Example.Handle", true, cancellationToken);
        Assert.Equal(2, feedRequests);
    }

    private static HttpResponseMessage HtmlResponse(string html) => new(HttpStatusCode.OK)
    {
        Content = new StringContent(html, Encoding.UTF8, "text/html"),
    };

    private static HttpResponseMessage XmlResponse(string xml) => new(HttpStatusCode.OK)
    {
        Content = new StringContent(xml, Encoding.UTF8, "application/xml"),
    };
}
