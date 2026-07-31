using System.Text.RegularExpressions;
using System.Text;
using Something.Domain.Exceptions;

namespace Something.Domain.Validation;

public static partial class ChannelRules
{
    public static string NormalizeTelegramUsername(string? value)
    {
        value = value?.Trim().TrimStart('@').ToLowerInvariant() ?? string.Empty;
        if (value.Length is < 5 or > 32 ||
            value.Any(static character =>
                (character is < 'a' or > 'z') &&
                (character is < '0' or > '9') &&
                character != '_'))
        {
            throw new ValidationException("Telegram channel must be a valid username.");
        }

        return value;
    }

    public static bool TryNormalizeYouTubeChannelId(string? value, out string channelId)
    {
        value = value?.Trim() ?? string.Empty;
        if (YouTubeChannelIdPattern().IsMatch(value))
        {
            channelId = value;
            return true;
        }

        channelId = string.Empty;
        return false;
    }

    public static string NormalizeYouTubeHandle(string? value)
    {
        value = value?.Trim().TrimStart('@') ?? string.Empty;
        var length = value.EnumerateRunes().Count();
        if (length is < 3 or > 30 || value.EnumerateRunes().Any(static character =>
                !Rune.IsLetterOrDigit(character) &&
                character.Value is not '_' and not '-' and not '.'))
        {
            throw new ValidationException("YouTube channel must be a valid channel ID or handle.");
        }

        return value;
    }

    public static (string Value, bool IsChannelId) NormalizeYouTubeReference(string? value)
    {
        if (TryNormalizeYouTubeChannelId(value, out var channelId))
        {
            return (channelId, true);
        }

        return (NormalizeYouTubeHandle(value), false);
    }

    public static bool IsValidYouTubeVideoId(string? value)
    {
        return value is not null && YouTubeVideoIdPattern().IsMatch(value.Trim());
    }

    [GeneratedRegex("^UC[A-Za-z0-9_-]{22}$", RegexOptions.CultureInvariant)]
    private static partial Regex YouTubeChannelIdPattern();

    [GeneratedRegex("^[A-Za-z0-9_-]{11}$", RegexOptions.CultureInvariant)]
    private static partial Regex YouTubeVideoIdPattern();
}
