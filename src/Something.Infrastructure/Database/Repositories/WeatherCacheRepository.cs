using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.Persistence;
using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;
using Something.Domain.Validation;

namespace Something.Infrastructure.Database.Repositories;

public sealed class WeatherCacheRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : IWeatherCacheRepository
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    public async Task<StoredWeatherLocation> GetAsync(CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            SELECT latitude, longitude, forecast_json, forecast_updated_at
            FROM location
            WHERE id = 1;
            """;
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        if (!await reader.ReadAsync(cancellationToken))
        {
            return new StoredWeatherLocation(false, 0, 0, null, null);
        }

        var latitude = reader.GetDouble(0);
        var longitude = reader.GetDouble(1);
        var forecast = reader.IsDBNull(2) ? null : DeserializeForecast(reader.GetString(2));
        var updatedAt = reader.IsDBNull(3) ? null : ParseTimestamp(reader.GetString(3));
        return new StoredWeatherLocation(true, latitude, longitude, forecast, updatedAt);
    }

    public Task<DateTimeOffset> SaveLocationAndForecastAsync(
        double latitude,
        double longitude,
        WeatherForecast forecast,
        CancellationToken cancellationToken = default)
    {
        WeatherRules.ValidateCoordinates(latitude, longitude);
        WeatherRules.ValidateForecast(forecast);
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                var updatedAt = DateTimeOffset.UtcNow;
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    INSERT INTO location (
                        id, latitude, longitude, forecast_json, forecast_updated_at
                    )
                    VALUES (1, $latitude, $longitude, $forecast, $updatedAt)
                    ON CONFLICT(id) DO UPDATE SET
                        latitude = excluded.latitude,
                        longitude = excluded.longitude,
                        forecast_json = excluded.forecast_json,
                        forecast_updated_at = excluded.forecast_updated_at,
                        updated_at = CURRENT_TIMESTAMP;
                    """;
                command.Parameters.AddWithValue("$latitude", latitude);
                command.Parameters.AddWithValue("$longitude", longitude);
                command.Parameters.AddWithValue("$forecast", SerializeForecast(forecast));
                command.Parameters.AddWithValue("$updatedAt", updatedAt.ToString("O", CultureInfo.InvariantCulture));
                await command.ExecuteNonQueryAsync(token);
                return updatedAt;
            },
            cancellationToken);
    }

    public Task<DateTimeOffset> SaveForecastAsync(
        WeatherForecast forecast,
        CancellationToken cancellationToken = default)
    {
        WeatherRules.ValidateForecast(forecast);
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                var updatedAt = DateTimeOffset.UtcNow;
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    UPDATE location
                    SET forecast_json = $forecast,
                        forecast_updated_at = $updatedAt,
                        updated_at = CURRENT_TIMESTAMP
                    WHERE id = 1;
                    """;
                command.Parameters.AddWithValue("$forecast", SerializeForecast(forecast));
                command.Parameters.AddWithValue("$updatedAt", updatedAt.ToString("O", CultureInfo.InvariantCulture));
                if (await command.ExecuteNonQueryAsync(token) != 1)
                {
                    throw new NotFoundException("The saved weather location was not found.");
                }

                return updatedAt;
            },
            cancellationToken);
    }

    private static string SerializeForecast(WeatherForecast forecast)
    {
        return JsonSerializer.Serialize(CachedForecast.FromDomain(forecast), JsonOptions);
    }

    private static WeatherForecast? DeserializeForecast(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
        {
            return null;
        }

        try
        {
            var cached = JsonSerializer.Deserialize<CachedForecast>(json, JsonOptions);
            var forecast = cached?.ToDomain();
            if (forecast is null)
            {
                return null;
            }

            WeatherRules.ValidateForecast(forecast);
            return forecast;
        }
        catch (Exception exception) when (exception is JsonException or ProviderUnavailableException)
        {
            return null;
        }
    }

    private static DateTimeOffset? ParseTimestamp(string value)
    {
        return DateTimeOffset.TryParse(
            value,
            CultureInfo.InvariantCulture,
            DateTimeStyles.AssumeUniversal,
            out var timestamp)
            ? timestamp
            : null;
    }

    private sealed record CachedForecast(
        [property: JsonPropertyName("current")] CachedCurrent Current,
        [property: JsonPropertyName("daily")] CachedDaily Daily,
        [property: JsonPropertyName("timezone")] string Timezone)
    {
        public static CachedForecast FromDomain(WeatherForecast forecast)
        {
            return new CachedForecast(
                new CachedCurrent(
                    forecast.Current.Temperature,
                    forecast.Current.ApparentTemperature,
                    forecast.Current.WeatherCode,
                    forecast.Current.WindSpeed),
                new CachedDaily(
                    forecast.Days.Select(static day => day.Date.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)).ToArray(),
                    forecast.Days.Select(static day => day.WeatherCode).ToArray(),
                    forecast.Days.Select(static day => day.MaximumTemperature).ToArray(),
                    forecast.Days.Select(static day => day.MinimumTemperature).ToArray(),
                    forecast.Days.Select(static day => day.PrecipitationProbability).ToArray()),
                forecast.Timezone);
        }

        public WeatherForecast? ToDomain()
        {
            var length = Daily.Time.Length;
            if (length == 0 || Daily.WeatherCode.Length != length ||
                Daily.MaximumTemperature.Length != length ||
                Daily.MinimumTemperature.Length != length ||
                Daily.PrecipitationProbability.Length != length)
            {
                return null;
            }

            var days = new WeatherDay[length];
            for (var index = 0; index < length; index++)
            {
                if (!DateOnly.TryParseExact(
                        Daily.Time[index], "yyyy-MM-dd", CultureInfo.InvariantCulture,
                        DateTimeStyles.None, out var date))
                {
                    return null;
                }

                days[index] = new WeatherDay(
                    date,
                    Daily.WeatherCode[index],
                    Daily.MaximumTemperature[index],
                    Daily.MinimumTemperature[index],
                    Daily.PrecipitationProbability[index]);
            }

            return new WeatherForecast(
                new WeatherCurrent(
                    Current.Temperature,
                    Current.ApparentTemperature,
                    Current.WeatherCode,
                    Current.WindSpeed),
                days,
                Timezone);
        }
    }

    private sealed record CachedCurrent(
        [property: JsonPropertyName("temperature_2m")] double Temperature,
        [property: JsonPropertyName("apparent_temperature")] double ApparentTemperature,
        [property: JsonPropertyName("weather_code")] int WeatherCode,
        [property: JsonPropertyName("wind_speed_10m")] double WindSpeed);

    private sealed record CachedDaily(
        [property: JsonPropertyName("time")] string[] Time,
        [property: JsonPropertyName("weather_code")] int[] WeatherCode,
        [property: JsonPropertyName("temperature_2m_max")] double[] MaximumTemperature,
        [property: JsonPropertyName("temperature_2m_min")] double[] MinimumTemperature,
        [property: JsonPropertyName("precipitation_probability_max")] double[] PrecipitationProbability);
}
