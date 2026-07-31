namespace Something.Domain.Models.Currencies;

public sealed record CurrencyRate(
    string Base,
    string Target,
    decimal Rate,
    DateOnly? Date,
    bool Found);
