using Something.Domain.Validation;

namespace Something.Domain.Models.Weather;

public sealed record WeatherDay(
    DateOnly Date,
    int WeatherCode,
    double MaximumTemperature,
    double MinimumTemperature,
    double PrecipitationProbability)
{
    public string Icon => WeatherRules.GetCodePresentation(WeatherCode).Icon;
    public string Condition => WeatherRules.GetCodePresentation(WeatherCode).Label;
    public string DateLabel => Date == DateOnly.FromDateTime(DateTime.Today)
        ? "Today"
        : Date.ToString("ddd, MMM d");
}
