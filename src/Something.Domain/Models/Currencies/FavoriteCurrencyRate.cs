namespace Something.Domain.Models.Currencies;

public sealed record FavoriteCurrencyRate(
    string Code,
    string Base,
    string Target,
    decimal Rate,
    bool Found)
{
    public string DisplayRate => Found ? Rate.ToString("0.####") : "Unavailable";
}
