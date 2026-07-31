using System.Text;
using Something.Domain.Exceptions;

namespace Something.Domain.Validation;

public static class NoteRules
{
    public const int MaximumTitleLength = 200;
    public const int MaximumBodyBytes = 256 * 1024;
    public const int MaximumSearchLength = 256;
    public const int DefaultListLimit = 50;
    public const int MaximumListLimit = 200;

    public static (string Title, string Body) NormalizeWrite(string? title, string? body)
    {
        title = title?.Trim() ?? string.Empty;
        body ??= string.Empty;

        if (title.EnumerateRunes().Count() > MaximumTitleLength)
        {
            throw new ValidationException("Note title must be 200 characters or fewer.");
        }

        if (Encoding.UTF8.GetByteCount(body) > MaximumBodyBytes)
        {
            throw new ValidationException("Note body must be 256 KiB or smaller.");
        }

        if (title.Length == 0 && string.IsNullOrWhiteSpace(body))
        {
            throw new ValidationException("A note needs a title or body.");
        }

        return (title, body);
    }

    public static string NormalizeSearch(string? query)
    {
        query = query?.Trim() ?? string.Empty;
        if (query.EnumerateRunes().Count() > MaximumSearchLength)
        {
            throw new ValidationException("Note search must be 256 characters or fewer.");
        }

        return query;
    }

    public static (int Limit, int Offset) NormalizePage(int limit, int offset)
    {
        if (offset < 0)
        {
            throw new ValidationException("Note offset cannot be negative.");
        }

        if (limit == 0)
        {
            limit = DefaultListLimit;
        }

        if (limit < 1 || limit > MaximumListLimit)
        {
            throw new ValidationException("Note limit must be between 1 and 200.");
        }

        return (limit, offset);
    }

    public static void RequirePositiveId(long id)
    {
        if (id <= 0)
        {
            throw new ValidationException("Note ID must be a positive integer.");
        }
    }

    public static void RequirePositiveRevision(int revision)
    {
        if (revision <= 0)
        {
            throw new ValidationException("Expected revision must be a positive integer.");
        }
    }

    public static string CreatePreview(string body)
    {
        var compact = string.Join(' ', body.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries));
        var runes = compact.EnumerateRunes().ToArray();
        return runes.Length <= 160
            ? compact
            : string.Concat(runes.Take(160).Select(static rune => rune.ToString())) + "…";
    }

    public static string GetDisplayTitle(string? title, string? bodyOrPreview)
    {
        if (!string.IsNullOrWhiteSpace(title))
        {
            return title.Trim();
        }

        var firstLine = (bodyOrPreview ?? string.Empty)
            .Split(['\r', '\n'], StringSplitOptions.RemoveEmptyEntries)
            .Select(static line => line.Trim())
            .FirstOrDefault(static line => line.Length > 0);
        return string.IsNullOrEmpty(firstLine) ? "Untitled note" : firstLine;
    }
}
