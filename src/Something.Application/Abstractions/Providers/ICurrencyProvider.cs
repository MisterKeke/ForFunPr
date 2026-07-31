using Something.Domain.Models.Currencies;

namespace Something.Application.Abstractions.Providers;

public interface ICurrencyProvider
{
    Task<CurrencyRate> GetRateAsync(
        string baseCode,
        string targetCode,
        CancellationToken cancellationToken = default);

    Task<AllCurrencyRates> GetAllRatesAsync(
        string baseCode,
        CancellationToken cancellationToken = default);
}
