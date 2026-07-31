using System.Globalization;
using System.Net;
using System.Text;
using AngleSharp.Dom;
using AngleSharp.Html.Parser;
using Something.Application.Abstractions.Providers;
using Something.Domain.Exceptions;
using Something.Domain.Models.Posts;
using Something.Domain.Validation;
using Something.Infrastructure.Caching;

namespace Something.Infrastructure.Providers.Telegram;

public sealed class TelegramProvider(IHttpClientFactory httpClientFactory) : ITelegramProvider
{
    public const string ClientName = "TelegramClient";
    private const int MaximumResponseBytes = 2 * 1024 * 1024;
    private const int MaximumPosts = 100;
    private readonly BoundedTtlCache<IReadOnlyList<TelegramPost>> _cache =
        new(128, TimeSpan.FromMinutes(5));

    public async Task<IReadOnlyList<TelegramPost>> GetPostsAsync(
        string username,
        int before = 0,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default)
    {
        username = ChannelRules.NormalizeTelegramUsername(username);
        if (before < 0)
        {
            throw new ValidationException("Telegram pagination cursor cannot be negative.");
        }

        if (before == 0 && forceRefresh)
        {
            _cache.Invalidate(username);
        }
        else if (before == 0 && _cache.TryGet(username, out var cached))
        {
            return cached.ToArray();
        }

        var builder = new UriBuilder(Uri.UriSchemeHttps, "t.me") { Path = $"/s/{username}" };
        if (before > 0)
        {
            builder.Query = $"before={before.ToString(CultureInfo.InvariantCulture)}";
        }

        using var response = await SendAsync(builder.Uri, cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("Telegram could not provide channel posts.");
        }

        var bytes = await BoundedHttpContent.ReadBytesAsync(
            response, MaximumResponseBytes, "Telegram", cancellationToken);
        var parser = new HtmlParser();
        var document = await parser.ParseDocumentAsync(Encoding.UTF8.GetString(bytes), cancellationToken);
        var posts = document.QuerySelectorAll(".tgme_widget_message_wrap")
            .Select(ParsePost)
            .Where(static post =>
                post.Text.Length > 0 || post.ImageUrls.Count > 0 ||
                post.PostId.Length > 0 || post.PublishedAt is not null)
            .Reverse()
            .DistinctBy(static post => post.PostId.Length > 0
                ? post.PostId
                : $"{post.PublishedAt:O}|{post.Text}", StringComparer.Ordinal)
            .OrderByDescending(static post => post.PublishedAt ?? DateTimeOffset.MinValue)
            .Take(MaximumPosts)
            .ToArray();

        if (before == 0 && posts.Length > 0)
        {
            _cache.Set(username, posts);
        }

        return posts.ToArray();
    }

    private async Task<HttpResponseMessage> SendAsync(Uri uri, CancellationToken cancellationToken)
    {
        if (uri.Scheme != Uri.UriSchemeHttps || uri.Host != "t.me")
        {
            throw new ProviderUnavailableException("Telegram is not configured securely.");
        }

        try
        {
            var client = httpClientFactory.CreateClient(ClientName);
            using var request = new HttpRequestMessage(HttpMethod.Get, uri);
            return await client.SendAsync(
                request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            throw new ProviderUnavailableException("Telegram timed out.");
        }
        catch (HttpRequestException exception)
        {
            throw new ProviderUnavailableException("Telegram is currently unavailable.", exception);
        }
    }

    private static TelegramPost ParsePost(IElement element)
    {
        var text = element.QuerySelector(".tgme_widget_message_text")?.TextContent.Trim() ?? string.Empty;
        var images = new List<string>();
        foreach (var photo in element.QuerySelectorAll(".tgme_widget_message_photo_wrap"))
        {
            var extracted = ExtractImageUrl(photo.GetAttribute("style") ?? string.Empty);
            if (IsSafeImageUrl(extracted))
            {
                images.Add(extracted);
            }
        }

        foreach (var image in element.QuerySelectorAll(".tgme_widget_message_photo img"))
        {
            var source = image.GetAttribute("src")?.Trim() ?? string.Empty;
            if (IsSafeImageUrl(source))
            {
                images.Add(source);
            }
        }

        var publishedValue = element.QuerySelector(".tgme_widget_message_date time")
            ?.GetAttribute("datetime");
        DateTimeOffset? published = DateTimeOffset.TryParse(
            publishedValue,
            CultureInfo.InvariantCulture,
            DateTimeStyles.AssumeUniversal,
            out var timestamp)
            ? timestamp
            : null;
        var views = element.QuerySelector(".tgme_widget_message_views")?.TextContent.Trim() ?? string.Empty;
        return new TelegramPost(
            text,
            images.Distinct(StringComparer.Ordinal).ToArray(),
            published,
            views,
            ExtractPostId(element));
    }

    private static string ExtractPostId(IElement element)
    {
        var direct = element.GetAttribute("data-post") ??
                     element.QuerySelector(".tgme_widget_message")?.GetAttribute("data-post");
        if (!string.IsNullOrWhiteSpace(direct))
        {
            return direct.Trim();
        }

        var href = element.QuerySelector(".tgme_widget_message_date")?.GetAttribute("href") ?? string.Empty;
        if (!Uri.TryCreate(href, UriKind.Absolute, out var uri) || uri.Host != "t.me")
        {
            return string.Empty;
        }

        var path = uri.AbsolutePath.Trim('/');
        return path.StartsWith("s/", StringComparison.Ordinal) ? path[2..] : path;
    }

    private static string ExtractImageUrl(string style)
    {
        var marker = style.IndexOf("url(", StringComparison.OrdinalIgnoreCase);
        if (marker < 0)
        {
            return string.Empty;
        }

        var value = style[(marker + 4)..];
        var end = value.IndexOf(')');
        return end < 0 ? string.Empty : value[..end].Trim().Trim('\'', '"');
    }

    private static bool IsSafeImageUrl(string value)
    {
        return Uri.TryCreate(value, UriKind.Absolute, out var uri) && uri.Scheme == Uri.UriSchemeHttps;
    }
}
