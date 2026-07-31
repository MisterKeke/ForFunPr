using Something.Domain.Exceptions;

namespace Something.Domain.Validation;

public static class CurrencyRules
{
    public static string NormalizeCode(string? value, string fieldName = "Currency")
    {
        value = value?.Trim().ToUpperInvariant() ?? string.Empty;
        if (value.Length != 3 || value.Any(static character => character is < 'A' or > 'Z'))
        {
            throw new ValidationException($"{fieldName} code must be three ASCII letters.");
        }

        return value;
    }

    public static (string Base, string Target) NormalizePair(string? value)
    {
        var parts = (value ?? string.Empty).Split(':');
        if (parts.Length != 2)
        {
            throw new ValidationException("Currency pair must use BASE:TARGET format.");
        }

        var baseCode = NormalizeCode(parts[0], "Base currency");
        var targetCode = NormalizeCode(parts[1], "Target currency");
        if (baseCode == targetCode)
        {
            throw new ValidationException("Favorite currencies must be different.");
        }

        return (baseCode, targetCode);
    }
}
