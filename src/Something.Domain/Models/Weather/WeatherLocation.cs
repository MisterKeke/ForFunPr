namespace Something.Domain.Models.Weather;

public sealed record WeatherLocation(
    string Name,
    string AdministrativeArea,
    string Country,
    double Latitude,
    double Longitude)
{
    public string DisplayName => string.Join(", ",
        new[] { Name, AdministrativeArea, Country }
            .Where(static value => !string.IsNullOrWhiteSpace(value))
            .Distinct(StringComparer.OrdinalIgnoreCase));
}
