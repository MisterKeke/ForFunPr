using System.Net;
using Something.Domain.Exceptions;
using Something.Infrastructure.Providers.Weather;

namespace Something.Infrastructure.Tests;

public sealed class WeatherTests
{
    [Fact]
    public async Task CoordinateForecastIsCachedAndRefreshOnlyReplacesItAfterSuccess()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await WeatherRepositoryTestHarness.CreateAsync(cancellationToken);

        Assert.False((await harness.Service.GetStoredAsync(cancellationToken)).Found);
        await harness.Service.GetForCoordinatesAsync(41.01, 28.97, cancellationToken);
        var stored = await harness.Service.GetStoredAsync(cancellationToken);
        Assert.True(stored.Found);
        Assert.Equal(20, stored.Forecast!.Current.Temperature);

        harness.Provider.Forecast = WeatherTestData.Forecast(24);
        var refreshed = await harness.Service.RefreshStoredAsync(cancellationToken);
        Assert.Equal(24, refreshed.Forecast!.Current.Temperature);

        harness.Provider.FailForecast = true;
        await Assert.ThrowsAsync<ProviderUnavailableException>(() =>
            harness.Service.RefreshStoredAsync(cancellationToken));
        Assert.Equal(
            24,
            (await harness.Service.GetStoredAsync(cancellationToken)).Forecast!.Current.Temperature);
    }

    [Fact]
    public async Task CitySearchUsesFirstLocationWithoutChangingSavedCoordinates()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await WeatherRepositoryTestHarness.CreateAsync(cancellationToken);

        var city = await harness.Service.GetForCityAsync(" Istanbul ", cancellationToken);
        Assert.True(city.Found);
        Assert.Equal("Istanbul, Türkiye", city.Location!.DisplayName);
        Assert.False((await harness.Service.GetStoredAsync(cancellationToken)).Found);

        harness.Provider.Location = null;
        Assert.False((await harness.Service.GetForCityAsync("Unknown", cancellationToken)).Found);
        await Assert.ThrowsAsync<ValidationException>(() =>
            harness.Service.GetForCoordinatesAsync(91, 0, cancellationToken));
    }

    [Fact]
    public async Task OpenMeteoProviderParsesForecastAndGeocodingContracts()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var handler = new DelegateHttpHandler((request, token) =>
        {
            token.ThrowIfCancellationRequested();
            var json = request.RequestUri!.Host.StartsWith("geocoding", StringComparison.Ordinal)
                ? "{\"results\":[{\"name\":\"Berlin\",\"latitude\":52.52,\"longitude\":13.41,\"admin1\":\"Berlin\",\"country\":\"Germany\"}]}"
                : "{\"timezone\":\"Europe/Berlin\",\"current\":{\"temperature_2m\":18.5,\"apparent_temperature\":17.2,\"weather_code\":2,\"wind_speed_10m\":10.4},\"daily\":{\"time\":[\"2026-07-31\"],\"weather_code\":[2],\"temperature_2m_max\":[22.1],\"temperature_2m_min\":[13.4],\"precipitation_probability_max\":[35]}}";
            return Task.FromResult(CurrencyTestsJsonResponse(HttpStatusCode.OK, json));
        });
        using var client = new HttpClient(handler);
        var provider = new OpenMeteoProvider(new StubHttpClientFactory(client));

        var location = await provider.FindCityAsync("Berlin", cancellationToken);
        Assert.Equal("Berlin, Germany", location!.DisplayName);
        var forecast = await provider.GetForecastAsync(location.Latitude, location.Longitude, cancellationToken);
        Assert.Equal("Europe/Berlin", forecast.Timezone);
        Assert.Equal(18.5, forecast.Current.Temperature);
        Assert.Equal("Partly cloudy", forecast.Current.Condition);
        Assert.Single(forecast.Days);
    }

    [Fact]
    public async Task OpenMeteoProviderRejectsIncompleteForecasts()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        const string json = "{\"timezone\":\"UTC\",\"current\":{\"temperature_2m\":10,\"apparent_temperature\":9,\"weather_code\":0,\"wind_speed_10m\":1},\"daily\":{\"time\":[\"2026-07-31\"],\"weather_code\":[],\"temperature_2m_max\":[12],\"temperature_2m_min\":[5],\"precipitation_probability_max\":[0]}}";
        var handler = new DelegateHttpHandler((_, _) =>
            Task.FromResult(CurrencyTestsJsonResponse(HttpStatusCode.OK, json)));
        using var client = new HttpClient(handler);
        var provider = new OpenMeteoProvider(new StubHttpClientFactory(client));

        await Assert.ThrowsAsync<ProviderUnavailableException>(() =>
            provider.GetForecastAsync(0, 0, cancellationToken));
    }

    private static HttpResponseMessage CurrencyTestsJsonResponse(HttpStatusCode status, string json)
    {
        return new HttpResponseMessage(status)
        {
            Content = new StringContent(json, System.Text.Encoding.UTF8, "application/json"),
        };
    }
}
