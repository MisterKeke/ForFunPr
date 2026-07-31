using System.Text;
using Something.Domain.Exceptions;

namespace Something.Domain.Validation;

public static class BookmarkRules
{
    public const int MaximumUrlBytes = 4096;
    public const int MaximumTitleLength = 200;
    public const int MaximumDescriptionBytes = 16 * 1024;
    public const int MaximumSearchLength = 256;
    public const int MaximumTags = 32;
    public const int MaximumTagLength = 64;
    public const int DefaultListLimit = 50;
    public const int MaximumListLimit = 200;

    public static (string DisplayUrl, string NormalizedUrl) NormalizeUrl(string? value)
    {
        value = value?.Trim() ?? string.Empty;
        if (value.Length == 0)
        {
            throw new ValidationException("Bookmark URL cannot be empty.");
        }

        if (Encoding.UTF8.GetByteCount(value) > MaximumUrlBytes)
        {
            throw new ValidationException("Bookmark URL must be 4096 bytes or fewer.");
        }

        if (value.Any(char.IsControl))
        {
            throw new ValidationException("Bookmark URL cannot contain control characters.");
        }

        if (!Uri.TryCreate(value, UriKind.Absolute, out var uri) ||
            (uri.Scheme != Uri.UriSchemeHttp && uri.Scheme != Uri.UriSchemeHttps))
        {
            throw new ValidationException("Bookmark URL must use HTTP or HTTPS.");
        }

        if (!string.IsNullOrEmpty(uri.UserInfo))
        {
            throw new ValidationException("Bookmark URL cannot contain credentials.");
        }

        if (string.IsNullOrWhiteSpace(uri.Host))
        {
            throw new ValidationException("Bookmark URL must include a hostname.");
        }

        var builder = new UriBuilder(uri)
        {
            Scheme = uri.Scheme.ToLowerInvariant(),
            Host = uri.Host.ToLowerInvariant(),
        };
        if ((builder.Scheme == Uri.UriSchemeHttp && builder.Port == 80) ||
            (builder.Scheme == Uri.UriSchemeHttps && builder.Port == 443))
        {
            builder.Port = -1;
        }

        var display = builder.Uri.AbsoluteUri;
        if (Encoding.UTF8.GetByteCount(display) > MaximumUrlBytes)
        {
            throw new ValidationException("Bookmark URL must be 4096 bytes or fewer.");
        }

        return (display, display);
    }

    public static (string Url, string NormalizedUrl, string Title, string Description, IReadOnlyList<string> Tags)
        NormalizeWrite(string? url, string? title, string? description, IReadOnlyList<string>? tags)
    {
        var (displayUrl, normalizedUrl) = NormalizeUrl(url);
        title = title?.Trim() ?? string.Empty;
        if (title.Length == 0)
        {
            throw new ValidationException("Bookmark title cannot be empty.");
        }

        if (title.EnumerateRunes().Count() > MaximumTitleLength)
        {
            throw new ValidationException("Bookmark title must be 200 characters or fewer.");
        }

        description = description?.Trim() ?? string.Empty;
        if (Encoding.UTF8.GetByteCount(description) > MaximumDescriptionBytes)
        {
            throw new ValidationException("Bookmark description must be 16 KiB or smaller.");
        }

        return (displayUrl, normalizedUrl, title, description, NormalizeTags(tags));
    }

    public static IReadOnlyList<string> NormalizeTags(IReadOnlyList<string>? values)
    {
        values ??= [];
        if (values.Count > MaximumTags)
        {
            throw new ValidationException("A bookmark can have at most 32 tags.");
        }

        var tags = new List<string>(values.Count);
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var value in values)
        {
            var tag = value?.Trim() ?? string.Empty;
            if (tag.Length == 0)
            {
                throw new ValidationException("Bookmark tags cannot be empty.");
            }

            if (tag.EnumerateRunes().Count() > MaximumTagLength)
            {
                throw new ValidationException("Bookmark tags must be 64 characters or fewer.");
            }

            if (seen.Add(tag))
            {
                tags.Add(tag);
            }
        }

        return tags;
    }

    public static string NormalizeSearch(string? query)
    {
        query = query?.Trim() ?? string.Empty;
        if (query.EnumerateRunes().Count() > MaximumSearchLength)
        {
            throw new ValidationException("Bookmark search must be 256 characters or fewer.");
        }

        return query;
    }

    public static (int Limit, int Offset) NormalizePage(int limit, int offset)
    {
        if (offset < 0)
        {
            throw new ValidationException("Bookmark offset cannot be negative.");
        }

        if (limit == 0)
        {
            limit = DefaultListLimit;
        }

        if (limit < 1 || limit > MaximumListLimit)
        {
            throw new ValidationException("Bookmark limit must be between 1 and 200.");
        }

        return (limit, offset);
    }

    public static void RequireMutation(long id, int revision)
    {
        RequirePositiveId(id);
        if (revision <= 0)
        {
            throw new ValidationException("Expected revision must be a positive integer.");
        }
    }

    public static void RequirePositiveId(long id)
    {
        if (id <= 0)
        {
            throw new ValidationException("Bookmark ID must be a positive integer.");
        }
    }
}
