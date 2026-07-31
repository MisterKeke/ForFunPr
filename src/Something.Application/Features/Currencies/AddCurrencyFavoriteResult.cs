namespace Something.Application.Features.Currencies;

public sealed record AddCurrencyFavoriteResult(string Pair, bool Added, bool Exists);
