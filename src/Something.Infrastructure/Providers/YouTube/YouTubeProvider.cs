using System.Globalization;
using System.Net;
using System.Text;
using System.Xml;
using System.Xml.Linq;
using AngleSharp.Dom;
using AngleSharp.Html.Parser;
using Something.Application.Abstractions.Providers;
using Something.Domain.Exceptions;
using Something.Domain.Models.Posts;
using Something.Domain.Validation;
using Something.Infrastructure.Caching;

namespace Something.Infrastructure.Providers.YouTube;

public sealed class YouTubeProvider(IHttpClientFactory httpClientFactory) : IYouTubeProvider
{
    public const string ClientName = "YouTubeClient";
    private const int MaximumResponseBytes = 2 * 1024 * 1024;
    private const int MaximumVideos = 50;
    private readonly BoundedTtlCache<IReadOnlyList<YouTubeVideo>> _videoCache =
        new(128, TimeSpan.FromMinutes(5));
    private readonly BoundedTtlCache<string> _handleCache =
        new(256, TimeSpan.FromMinutes(5));

    public async Task<(string ChannelId, string Handle)> ResolveChannelAsync(
        string reference,
        CancellationToken cancellationToken = default)
    {
        var (normalized, isChannelId) = ChannelRules.NormalizeYouTubeReference(reference);
        if (isChannelId)
        {
            return (normalized, string.Empty);
        }

        var cacheKey = normalized.ToLowerInvariant();
        if (_handleCache.TryGet(cacheKey, out var cached))
        {
            return (cached, normalized);
        }

        var uri = new UriBuilder(Uri.UriSchemeHttps, "www.youtube.com")
        {
            Path = $"/@{normalized}",
        }.Uri;
        using var response = await SendAsync(uri, "text/html", cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("YouTube could not resolve this channel handle.");
        }

        var bytes = await BoundedHttpContent.ReadBytesAsync(
            response, MaximumResponseBytes, "YouTube", cancellationToken);
        var parser = new HtmlParser();
        var document = await parser.ParseDocumentAsync(Encoding.UTF8.GetString(bytes), cancellationToken);
        var channelId = ExtractChannelId(document);
        if (channelId.Length == 0)
        {
            throw new ProviderUnavailableException("YouTube did not return a valid channel ID.");
        }

        _handleCache.Set(cacheKey, channelId);
        return (channelId, normalized);
    }

    public async Task<IReadOnlyList<YouTubeVideo>> GetVideosAsync(
        string reference,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default)
    {
        var (channelId, _) = await ResolveChannelAsync(reference, cancellationToken);
        if (forceRefresh)
        {
            _videoCache.Invalidate(channelId);
        }
        else if (_videoCache.TryGet(channelId, out var cached))
        {
            return cached.ToArray();
        }

        var builder = new UriBuilder(Uri.UriSchemeHttps, "www.youtube.com")
        {
            Path = "/feeds/videos.xml",
            Query = $"channel_id={Uri.EscapeDataString(channelId)}",
        };
        using var response = await SendAsync(builder.Uri, "application/xml", cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("YouTube could not provide this channel feed.");
        }

        var bytes = await BoundedHttpContent.ReadBytesAsync(
            response, MaximumResponseBytes, "YouTube", cancellationToken);
        IReadOnlyList<YouTubeVideo> videos;
        try
        {
            videos = ParseFeed(bytes, channelId);
        }
        catch (XmlException exception)
        {
            throw new ProviderUnavailableException("YouTube returned an invalid channel feed.", exception);
        }

        if (videos.Count > 0)
        {
            _videoCache.Set(channelId, videos);
        }

        return videos.ToArray();
    }

    private async Task<HttpResponseMessage> SendAsync(
        Uri uri,
        string accept,
        CancellationToken cancellationToken)
    {
        if (uri.Scheme != Uri.UriSchemeHttps || uri.Host != "www.youtube.com")
        {
            throw new ProviderUnavailableException("YouTube is not configured securely.");
        }

        try
        {
            var client = httpClientFactory.CreateClient(ClientName);
            using var request = new HttpRequestMessage(HttpMethod.Get, uri);
            request.Headers.Accept.ParseAdd(accept);
            return await client.SendAsync(
                request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            throw new ProviderUnavailableException("YouTube timed out.");
        }
        catch (HttpRequestException exception)
        {
            throw new ProviderUnavailableException("YouTube is currently unavailable.", exception);
        }
    }

    private static string ExtractChannelId(IDocument document)
    {
        var candidates = new List<string?>
        {
            document.QuerySelector("meta[itemprop='channelId']")?.GetAttribute("content"),
            ChannelIdFromUrl(document.QuerySelector("link[rel='canonical']")?.GetAttribute("href")),
            ChannelIdFromUrl(document.QuerySelector("meta[property='og:url']")?.GetAttribute("content")),
        };
        foreach (var script in document.QuerySelectorAll("script"))
        {
            foreach (var pattern in new[] { "\"channelId\":\"", "\"channel_id\":\"", "\"externalChannelId\":\"" })
            {
                var start = script.TextContent.IndexOf(pattern, StringComparison.Ordinal);
                if (start < 0)
                {
                    continue;
                }

                start += pattern.Length;
                var end = script.TextContent.IndexOf('"', start);
                if (end > start)
                {
                    candidates.Add(script.TextContent[start..end]);
                }
            }
        }

        return candidates
            .Select(static value => value?.Trim() ?? string.Empty)
            .FirstOrDefault(static value => ChannelRules.TryNormalizeYouTubeChannelId(value, out _))
            ?? string.Empty;
    }

    private static string ChannelIdFromUrl(string? value)
    {
        if (!Uri.TryCreate(value, UriKind.Absolute, out var uri))
        {
            return string.Empty;
        }

        var segments = uri.AbsolutePath.Split('/', StringSplitOptions.RemoveEmptyEntries);
        for (var index = 0; index + 1 < segments.Length; index++)
        {
            if (segments[index] == "channel")
            {
                return segments[index + 1];
            }
        }

        return string.Empty;
    }

    private static IReadOnlyList<YouTubeVideo> ParseFeed(byte[] bytes, string channelId)
    {
        using var stream = new MemoryStream(bytes, writable: false);
        using var reader = XmlReader.Create(stream, new XmlReaderSettings
        {
            DtdProcessing = DtdProcessing.Prohibit,
            XmlResolver = null,
        });
        var document = XDocument.Load(reader, LoadOptions.None);
        XNamespace atom = "http://www.w3.org/2005/Atom";
        XNamespace media = "http://search.yahoo.com/mrss/";
        XNamespace yt = "http://www.youtube.com/xml/schemas/2015";

        return document.Root?
            .Elements(atom + "entry")
            .Select(entry =>
            {
                var videoId = entry.Element(yt + "videoId")?.Value.Trim() ?? string.Empty;
                if (!ChannelRules.IsValidYouTubeVideoId(videoId))
                {
                    return null;
                }

                var group = entry.Element(media + "group");
                var publishedValue = entry.Element(atom + "published")?.Value;
                DateTimeOffset? published = DateTimeOffset.TryParse(
                    publishedValue,
                    CultureInfo.InvariantCulture,
                    DateTimeStyles.AssumeUniversal,
                    out var timestamp)
                    ? timestamp
                    : null;
                return new YouTubeVideo(
                    videoId,
                    entry.Element(atom + "title")?.Value.Trim() ?? string.Empty,
                    group?.Element(media + "description")?.Value.Trim() ?? string.Empty,
                    group?.Element(media + "thumbnail")?.Attribute("url")?.Value.Trim() ?? string.Empty,
                    published,
                    channelId,
                    entry.Element(atom + "author")?.Element(atom + "name")?.Value.Trim() ?? string.Empty,
                    $"https://www.youtube.com/watch?v={Uri.EscapeDataString(videoId)}",
                    group?.Element(media + "community")?.Element(media + "statistics")
                        ?.Attribute("views")?.Value.Trim() ?? string.Empty,
                    string.Empty);
            })
            .Where(static video => video is not null)
            .Cast<YouTubeVideo>()
            .DistinctBy(static video => video.VideoId, StringComparer.Ordinal)
            .OrderByDescending(static video => video.PublishedAt ?? DateTimeOffset.MinValue)
            .Take(MaximumVideos)
            .ToArray()
            ?? [];
    }
}
