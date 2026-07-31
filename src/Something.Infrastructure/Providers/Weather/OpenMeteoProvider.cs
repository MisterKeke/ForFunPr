using System.Globalization;
using System.Net;
using System.Text.Json.Serialization;
using Something.Application.Abstractions.Providers;
using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;
using Something.Domain.Validation;

namespace Something.Infrastructure.Providers.Weather;

public sealed class OpenMeteoProvider(IHttpClientFactory httpClientFactory) : IWeatherProvider
{
    public const string ClientName = "OpenMeteoClient";
    private const int MaximumResponseBytes = 2 * 1024 * 1024;

    public async Task<WeatherForecast> GetForecastAsync(
        double latitude,
        double longitude,
        CancellationToken cancellationToken = default)
    {
        WeatherRules.ValidateCoordinates(latitude, longitude);
        var uri = BuildForecastUri(latitude, longitude);
        using var response = await SendAsync(uri, cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("Open-Meteo could not provide a forecast.");
        }

        var data = await BoundedHttpContent.ReadJsonAsync<ForecastResponse>(
            response, MaximumResponseBytes, "Open-Meteo", cancellationToken);
        return MapForecast(data);
    }

    public async Task<WeatherLocation?> FindCityAsync(
        string city,
        CancellationToken cancellationToken = default)
    {
        var uri = BuildGeocodingUri(city);
        using var response = await SendAsync(uri, cancellationToken);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw new ProviderUnavailableException("Open-Meteo could not search for this city.");
        }

        var data = await BoundedHttpContent.ReadJsonAsync<SearchResponse>(
            response, MaximumResponseBytes, "Open-Meteo", cancellationToken);
        var match = data.Results?.FirstOrDefault();
        if (match is null)
        {
            return null;
        }

        WeatherRules.ValidateCoordinates(match.Latitude, match.Longitude);
        return new WeatherLocation(
            match.Name ?? string.Empty,
            match.AdministrativeArea ?? string.Empty,
            match.Country ?? string.Empty,
            match.Latitude,
            match.Longitude);
    }

    private async Task<HttpResponseMessage> SendAsync(Uri uri, CancellationToken cancellationToken)
    {
        if (uri.Scheme != Uri.UriSchemeHttps ||
            (uri.Host != "api.open-meteo.com" && uri.Host != "geocoding-api.open-meteo.com"))
        {
            throw new ProviderUnavailableException("Open-Meteo is not configured securely.");
        }

        try
        {
            var client = httpClientFactory.CreateClient(ClientName);
            using var request = new HttpRequestMessage(HttpMethod.Get, uri);
            return await client.SendAsync(
                request,
                HttpCompletionOption.ResponseHeadersRead,
                cancellationToken);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            throw new ProviderUnavailableException("Open-Meteo timed out.");
        }
        catch (HttpRequestException exception)
        {
            throw new ProviderUnavailableException("Open-Meteo is currently unavailable.", exception);
        }
    }

    private static WeatherForecast MapForecast(ForecastResponse response)
    {
        var daily = response.Daily;
        var length = daily?.Time?.Length ?? 0;
        if (response.Current is null || daily is null || length == 0 ||
            daily.WeatherCode?.Length != length ||
            daily.MaximumTemperature?.Length != length ||
            daily.MinimumTemperature?.Length != length ||
            daily.PrecipitationProbability?.Length != length)
        {
            throw new ProviderUnavailableException("Open-Meteo returned incomplete daily weather data.");
        }

        var times = daily.Time!;
        var weatherCodes = daily.WeatherCode!;
        var maximumTemperatures = daily.MaximumTemperature!;
        var minimumTemperatures = daily.MinimumTemperature!;
        var precipitationProbabilities = daily.PrecipitationProbability!;
        var days = new WeatherDay[length];
        for (var index = 0; index < length; index++)
        {
            if (!DateOnly.TryParseExact(
                    times[index],
                    "yyyy-MM-dd",
                    CultureInfo.InvariantCulture,
                    DateTimeStyles.None,
                    out var date))
            {
                throw new ProviderUnavailableException("Open-Meteo returned an invalid forecast date.");
            }

            days[index] = new WeatherDay(
                date,
                weatherCodes[index],
                maximumTemperatures[index],
                minimumTemperatures[index],
                precipitationProbabilities[index]);
        }

        var forecast = new WeatherForecast(
            new WeatherCurrent(
                response.Current.Temperature,
                response.Current.ApparentTemperature,
                response.Current.WeatherCode,
                response.Current.WindSpeed),
            days,
            response.Timezone ?? string.Empty);
        WeatherRules.ValidateForecast(forecast);
        return forecast;
    }

    private static Uri BuildForecastUri(double latitude, double longitude)
    {
        var query = new Dictionary<string, string>
        {
            ["latitude"] = latitude.ToString("0.000000", CultureInfo.InvariantCulture),
            ["longitude"] = longitude.ToString("0.000000", CultureInfo.InvariantCulture),
            ["current"] = "temperature_2m,apparent_temperature,weather_code,wind_speed_10m",
            ["daily"] = "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max",
            ["timezone"] = "auto",
            ["forecast_days"] = WeatherRules.ForecastDays.ToString(CultureInfo.InvariantCulture),
        };
        return BuildUri("api.open-meteo.com", "/v1/forecast", query);
    }

    private static Uri BuildGeocodingUri(string city)
    {
        return BuildUri(
            "geocoding-api.open-meteo.com",
            "/v1/search",
            new Dictionary<string, string>
            {
                ["name"] = city,
                ["count"] = "1",
                ["language"] = "en",
                ["format"] = "json",
            });
    }

    private static Uri BuildUri(string host, string path, IReadOnlyDictionary<string, string> query)
    {
        var builder = new UriBuilder(Uri.UriSchemeHttps, host) { Path = path };
        builder.Query = string.Join("&", query.Select(static pair =>
            $"{Uri.EscapeDataString(pair.Key)}={Uri.EscapeDataString(pair.Value)}"));
        return builder.Uri;
    }

    private sealed record ForecastResponse(
        [property: JsonPropertyName("current")] CurrentResponse? Current,
        [property: JsonPropertyName("daily")] DailyResponse? Daily,
        [property: JsonPropertyName("timezone")] string? Timezone);

    private sealed record CurrentResponse(
        [property: JsonPropertyName("temperature_2m")] double Temperature,
        [property: JsonPropertyName("apparent_temperature")] double ApparentTemperature,
        [property: JsonPropertyName("weather_code")] int WeatherCode,
        [property: JsonPropertyName("wind_speed_10m")] double WindSpeed);

    private sealed record DailyResponse(
        [property: JsonPropertyName("time")] string[]? Time,
        [property: JsonPropertyName("weather_code")] int[]? WeatherCode,
        [property: JsonPropertyName("temperature_2m_max")] double[]? MaximumTemperature,
        [property: JsonPropertyName("temperature_2m_min")] double[]? MinimumTemperature,
        [property: JsonPropertyName("precipitation_probability_max")] double[]? PrecipitationProbability);

    private sealed record SearchResponse(
        [property: JsonPropertyName("results")] SearchResult[]? Results);

    private sealed record SearchResult(
        [property: JsonPropertyName("name")] string? Name,
        [property: JsonPropertyName("latitude")] double Latitude,
        [property: JsonPropertyName("longitude")] double Longitude,
        [property: JsonPropertyName("admin1")] string? AdministrativeArea,
        [property: JsonPropertyName("country")] string? Country);
}
