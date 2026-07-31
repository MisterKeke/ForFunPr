using Something.Domain.Models.Currencies;

namespace Something.Application.Features.Currencies;

public interface ICurrencyService
{
    Task<CurrencyRate> GetRateAsync(string baseCode, string targetCode, CancellationToken cancellationToken = default);
    Task<AllCurrencyRates> GetAllRatesAsync(string baseCode, CancellationToken cancellationToken = default);
    Task<AddCurrencyFavoriteResult> AddFavoriteAsync(string pair, CancellationToken cancellationToken = default);
    Task<string> RemoveFavoriteAsync(string pair, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<string>> ListFavoritesAsync(CancellationToken cancellationToken = default);
    Task<IReadOnlyList<FavoriteCurrencyRate>> GetFavoritesWithRatesAsync(CancellationToken cancellationToken = default);
}
