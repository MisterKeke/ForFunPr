using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;

namespace Something.Domain.Validation;

public static class WeatherRules
{
    public const int ForecastDays = 6;

    public static void ValidateCoordinates(double latitude, double longitude)
    {
        if (!double.IsFinite(latitude) || !double.IsFinite(longitude) ||
            latitude is < -90 or > 90 || longitude is < -180 or > 180)
        {
            throw new ValidationException(
                "Latitude must be between -90 and 90 and longitude must be between -180 and 180.");
        }
    }

    public static string NormalizeCity(string? city)
    {
        city = city?.Trim() ?? string.Empty;
        var length = city.EnumerateRunes().Count();
        if (length is < 1 or > 100)
        {
            throw new ValidationException("City name must contain between 1 and 100 characters.");
        }

        return city;
    }

    public static void ValidateForecast(WeatherForecast? forecast)
    {
        if (forecast is null || forecast.Current is null || forecast.Days.Count == 0)
        {
            throw new ProviderUnavailableException("The weather forecast is incomplete.");
        }

        if (!double.IsFinite(forecast.Current.Temperature) ||
            !double.IsFinite(forecast.Current.ApparentTemperature) ||
            !double.IsFinite(forecast.Current.WindSpeed))
        {
            throw new ProviderUnavailableException("The current weather contains invalid values.");
        }

        foreach (var day in forecast.Days)
        {
            if (!double.IsFinite(day.MaximumTemperature) ||
                !double.IsFinite(day.MinimumTemperature) ||
                !double.IsFinite(day.PrecipitationProbability))
            {
                throw new ProviderUnavailableException("The weather forecast contains invalid values.");
            }
        }
    }

    public static (string Icon, string Label) GetCodePresentation(int code)
    {
        return code switch
        {
            0 => ("☀️", "Clear sky"),
            1 or 2 => ("🌤️", "Partly cloudy"),
            3 => ("☁️", "Overcast"),
            45 or 48 => ("🌫️", "Foggy"),
            51 or 53 or 55 => ("🌦️", "Drizzle"),
            56 or 57 => ("🌧️", "Freezing drizzle"),
            61 or 63 or 65 => ("🌧️", "Rain"),
            66 or 67 => ("🌧️", "Freezing rain"),
            71 or 73 or 75 or 77 => ("❄️", "Snow"),
            80 or 81 or 82 => ("🌦️", "Rain showers"),
            85 or 86 => ("🌨️", "Snow showers"),
            95 => ("⛈️", "Thunderstorm"),
            96 or 99 => ("⛈️", "Thunderstorm with hail"),
            _ => ("🌡️", "Weather unavailable"),
        };
    }
}
