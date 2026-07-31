using Something.Domain.Enums;
using Something.Domain.Exceptions;

namespace Something.Domain.Validation;

public static class FavoriteRules
{
    public static string NormalizeCategoryName(string? name)
    {
        name = name?.Trim() ?? string.Empty;
        if (name.Length == 0)
        {
            throw new ValidationException("Category name cannot be empty.");
        }

        return name;
    }

    public static string ToDatabase(FavoriteSource source)
    {
        return source switch
        {
            FavoriteSource.Telegram => "telegram",
            FavoriteSource.YouTube => "youtube",
            _ => throw new ValidationException("Source must be Telegram or YouTube."),
        };
    }

    public static FavoriteSource ParseSource(string value)
    {
        return value.Trim().ToLowerInvariant() switch
        {
            "telegram" => FavoriteSource.Telegram,
            "youtube" => FavoriteSource.YouTube,
            _ => throw new ValidationException("Source must be Telegram or YouTube."),
        };
    }

    public static void RequirePositiveCategoryId(long id)
    {
        if (id <= 0)
        {
            throw new ValidationException("Category ID must be a positive integer.");
        }
    }

    public static string CategoryKey(FavoriteSource source, string name)
    {
        return $"{ToDatabase(source)}:{NormalizeCategoryName(name).ToLowerInvariant()}";
    }
}
