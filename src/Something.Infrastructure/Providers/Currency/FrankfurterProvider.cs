using System.Globalization;
using System.Net;
using System.Net.Http.Headers;
using System.Text.Json.Serialization;
using Something.Application.Abstractions.Providers;
using Something.Domain.Exceptions;
using Something.Domain.Models.Currencies;

namespace Something.Infrastructure.Providers.Currency;

public sealed class FrankfurterProvider(IHttpClientFactory httpClientFactory) : ICurrencyProvider
{
    public const string ClientName = "FrankfurterClient";
    private const int MaximumResponseBytes = 1024 * 1024;

    public async Task<CurrencyRate> GetRateAsync(
        string baseCode,
        string targetCode,
        CancellationToken cancellationToken = default)
    {
        using var request = new HttpRequestMessage(
            HttpMethod.Get,
            $"v2/rate/{Uri.EscapeDataString(baseCode)}/{Uri.EscapeDataString(targetCode)}");
        using var response = await SendAsync(request, cancellationToken);
        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return new CurrencyRate(baseCode, targetCode, 0m, null, false);
        }

        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("Frankfurter could not provide this exchange rate.");
        }

        var data = await BoundedHttpContent.ReadJsonAsync<RateResponse>(
            response, MaximumResponseBytes, "Frankfurter", cancellationToken);
        if (!string.Equals(data.Base, baseCode, StringComparison.Ordinal) ||
            !string.Equals(data.Quote, targetCode, StringComparison.Ordinal) ||
            data.Rate <= 0)
        {
            throw new ProviderUnavailableException("Frankfurter returned a mismatched exchange rate.");
        }

        return new CurrencyRate(
            data.Base,
            data.Quote,
            data.Rate,
            ParseDate(data.Date),
            true);
    }

    public async Task<AllCurrencyRates> GetAllRatesAsync(
        string baseCode,
        CancellationToken cancellationToken = default)
    {
        using var request = new HttpRequestMessage(
            HttpMethod.Get,
            $"v2/rates?base={Uri.EscapeDataString(baseCode)}");
        using var response = await SendAsync(request, cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("Frankfurter could not provide exchange rates.");
        }

        var data = await BoundedHttpContent.ReadJsonAsync<RateResponse[]>(
            response, MaximumResponseBytes, "Frankfurter", cancellationToken);
        if (data.Any(item =>
                !string.Equals(item.Base, baseCode, StringComparison.Ordinal) ||
                string.IsNullOrWhiteSpace(item.Quote) ||
                item.Rate <= 0))
        {
            throw new ProviderUnavailableException("Frankfurter returned mismatched exchange rates.");
        }

        var rates = data
            .GroupBy(static item => item.Quote, StringComparer.Ordinal)
            .Select(static group => group.Last())
            .OrderBy(static item => item.Quote, StringComparer.Ordinal)
            .Select(static item => new CurrencyRateItem(item.Quote, item.Rate))
            .ToArray();
        return new AllCurrencyRates(
            baseCode,
            data.Length == 0 ? null : ParseDate(data[0].Date),
            rates);
    }

    private async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        try
        {
            var client = httpClientFactory.CreateClient(ClientName);
            if (client.BaseAddress?.Scheme != Uri.UriSchemeHttps)
            {
                throw new ProviderUnavailableException("Frankfurter is not configured securely.");
            }

            return await client.SendAsync(
                request,
                HttpCompletionOption.ResponseHeadersRead,
                cancellationToken);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            throw new ProviderUnavailableException("Frankfurter timed out.");
        }
        catch (HttpRequestException exception)
        {
            throw new ProviderUnavailableException("Frankfurter is currently unavailable.", exception);
        }
    }

    private static DateOnly? ParseDate(string value)
    {
        return DateOnly.TryParseExact(
            value,
            "yyyy-MM-dd",
            CultureInfo.InvariantCulture,
            DateTimeStyles.None,
            out var date)
            ? date
            : null;
    }

    private sealed record RateResponse(
        [property: JsonPropertyName("date")] string Date,
        [property: JsonPropertyName("base")] string Base,
        [property: JsonPropertyName("quote")] string Quote,
        [property: JsonPropertyName("rate")] decimal Rate);
}
