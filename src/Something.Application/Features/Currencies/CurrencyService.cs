using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Abstractions.Providers;
using Something.Application.Messages;
using Something.Domain.Models.Currencies;
using Something.Domain.Validation;

namespace Something.Application.Features.Currencies;

public sealed class CurrencyService(
    ICurrencyProvider provider,
    ICurrencyFavoriteRepository favorites,
    IMessenger messenger) : ICurrencyService
{
    private const int FavoriteRefreshConcurrency = 4;

    public Task<CurrencyRate> GetRateAsync(
        string baseCode,
        string targetCode,
        CancellationToken cancellationToken = default)
    {
        baseCode = CurrencyRules.NormalizeCode(baseCode, "Base currency");
        targetCode = CurrencyRules.NormalizeCode(targetCode, "Target currency");
        return baseCode == targetCode
            ? Task.FromResult(new CurrencyRate(baseCode, targetCode, 1m, null, true))
            : provider.GetRateAsync(baseCode, targetCode, cancellationToken);
    }

    public Task<AllCurrencyRates> GetAllRatesAsync(
        string baseCode,
        CancellationToken cancellationToken = default)
    {
        baseCode = CurrencyRules.NormalizeCode(baseCode, "Base currency");
        return provider.GetAllRatesAsync(baseCode, cancellationToken);
    }

    public async Task<AddCurrencyFavoriteResult> AddFavoriteAsync(
        string pair,
        CancellationToken cancellationToken = default)
    {
        var (baseCode, targetCode) = CurrencyRules.NormalizePair(pair);
        var added = await favorites.AddAsync(baseCode, targetCode, cancellationToken);
        if (added)
        {
            PublishChanged();
        }

        return new AddCurrencyFavoriteResult($"{baseCode}:{targetCode}", added, !added);
    }

    public async Task<string> RemoveFavoriteAsync(
        string pair,
        CancellationToken cancellationToken = default)
    {
        var (baseCode, targetCode) = CurrencyRules.NormalizePair(pair);
        if (await favorites.RemoveAsync(baseCode, targetCode, cancellationToken))
        {
            PublishChanged();
        }

        return $"{baseCode}:{targetCode}";
    }

    public Task<IReadOnlyList<string>> ListFavoritesAsync(CancellationToken cancellationToken = default)
    {
        return favorites.ListAsync(cancellationToken);
    }

    public async Task<IReadOnlyList<FavoriteCurrencyRate>> GetFavoritesWithRatesAsync(
        CancellationToken cancellationToken = default)
    {
        var pairs = await favorites.ListAsync(cancellationToken);
        var results = new FavoriteCurrencyRate[pairs.Count];
        await Parallel.ForEachAsync(
            Enumerable.Range(0, pairs.Count),
            new ParallelOptions
            {
                CancellationToken = cancellationToken,
                MaxDegreeOfParallelism = FavoriteRefreshConcurrency,
            },
            async (index, token) =>
            {
                var pair = pairs[index];
                var (baseCode, targetCode) = CurrencyRules.NormalizePair(pair);
                try
                {
                    var rate = await GetRateAsync(baseCode, targetCode, token);
                    results[index] = new FavoriteCurrencyRate(
                        pair, baseCode, targetCode, rate.Rate, rate.Found);
                }
                catch (Domain.Exceptions.ProviderUnavailableException)
                {
                    results[index] = new FavoriteCurrencyRate(pair, baseCode, targetCode, 0m, false);
                }
            });
        return results;
    }

    private void PublishChanged()
    {
        messenger.Send(new FavoritesChangedMessage("currency", DateTimeOffset.UtcNow));
    }
}
