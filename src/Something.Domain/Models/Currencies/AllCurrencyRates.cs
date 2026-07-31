namespace Something.Domain.Models.Currencies;

public sealed record AllCurrencyRates(
    string Base,
    DateOnly? Date,
    IReadOnlyList<CurrencyRateItem> Rates);
