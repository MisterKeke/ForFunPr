using System.Net;
using System.Text;
using Something.Domain.Exceptions;
using Something.Infrastructure.Providers.Currency;

namespace Something.Infrastructure.Tests;

public sealed class CurrencyTests
{
    [Fact]
    public async Task FrankfurterProviderParsesSingleAndAllRateContracts()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var handler = new DelegateHttpHandler((request, token) =>
        {
            token.ThrowIfCancellationRequested();
            var json = request.RequestUri!.AbsolutePath.Contains("/rate/", StringComparison.Ordinal)
                ? "{\"date\":\"2026-07-31\",\"base\":\"EUR\",\"quote\":\"USD\",\"rate\":1.15}"
                : "[{\"date\":\"2026-07-31\",\"base\":\"EUR\",\"quote\":\"JPY\",\"rate\":170.2},{\"date\":\"2026-07-31\",\"base\":\"EUR\",\"quote\":\"USD\",\"rate\":1.15}]";
            return Task.FromResult(JsonResponse(HttpStatusCode.OK, json));
        });
        using var client = new HttpClient(handler) { BaseAddress = new Uri("https://api.frankfurter.dev/") };
        var provider = new FrankfurterProvider(new StubHttpClientFactory(client));

        var rate = await provider.GetRateAsync("EUR", "USD", cancellationToken);
        Assert.True(rate.Found);
        Assert.Equal(1.15m, rate.Rate);
        Assert.Equal(new DateOnly(2026, 7, 31), rate.Date);

        var all = await provider.GetAllRatesAsync("EUR", cancellationToken);
        Assert.Equal(["JPY", "USD"], all.Rates.Select(static item => item.Code));
    }

    [Fact]
    public async Task FrankfurterProviderHandlesNotFoundMismatchAndOversizeResponses()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var responses = new Queue<HttpResponseMessage>(
        [
            JsonResponse(HttpStatusCode.NotFound, "{}"),
            JsonResponse(HttpStatusCode.OK, "{\"date\":\"2026-07-31\",\"base\":\"GBP\",\"quote\":\"USD\",\"rate\":1.2}"),
            JsonResponse(HttpStatusCode.OK, "[" + new string(' ', 1024 * 1024) + "]"),
        ]);
        var handler = new DelegateHttpHandler((_, _) => Task.FromResult(responses.Dequeue()));
        using var client = new HttpClient(handler) { BaseAddress = new Uri("https://api.frankfurter.dev/") };
        var provider = new FrankfurterProvider(new StubHttpClientFactory(client));

        Assert.False((await provider.GetRateAsync("EUR", "USD", cancellationToken)).Found);
        await Assert.ThrowsAsync<ProviderUnavailableException>(() =>
            provider.GetRateAsync("EUR", "USD", cancellationToken));
        await Assert.ThrowsAsync<ProviderUnavailableException>(() =>
            provider.GetAllRatesAsync("EUR", cancellationToken));
    }

    [Fact]
    public async Task FavoritePairsAreValidatedStoredAndRefreshed()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await CurrencyRepositoryTestHarness.CreateAsync(cancellationToken);

        var added = await harness.Service.AddFavoriteAsync(" eur:usd ", cancellationToken);
        Assert.True(added.Added);
        Assert.Equal("EUR:USD", added.Pair);
        Assert.True((await harness.Service.AddFavoriteAsync("EUR:USD", cancellationToken)).Exists);
        Assert.Equal(["EUR:USD"], await harness.Service.ListFavoritesAsync(cancellationToken));

        var favorite = Assert.Single(await harness.Service.GetFavoritesWithRatesAsync(cancellationToken));
        Assert.True(favorite.Found);
        Assert.Equal(1.25m, favorite.Rate);

        Assert.Equal("EUR:USD", await harness.Service.RemoveFavoriteAsync("eur:usd", cancellationToken));
        Assert.Empty(await harness.Service.ListFavoritesAsync(cancellationToken));
    }

    [Fact]
    public async Task SameCurrencyRateIsLocalAndInvalidCodesAreRejected()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await CurrencyRepositoryTestHarness.CreateAsync(cancellationToken);

        var same = await harness.Service.GetRateAsync("try", "TRY", cancellationToken);
        Assert.Equal(1m, same.Rate);
        Assert.Null(same.Date);
        await Assert.ThrowsAsync<ValidationException>(() =>
            harness.Service.GetRateAsync("EU", "USD", cancellationToken));
        await Assert.ThrowsAsync<ValidationException>(() =>
            harness.Service.AddFavoriteAsync("USD:USD", cancellationToken));
    }

    private static HttpResponseMessage JsonResponse(HttpStatusCode status, string json)
    {
        return new HttpResponseMessage(status)
        {
            Content = new StringContent(json, Encoding.UTF8, "application/json"),
        };
    }
}
