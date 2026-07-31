using Something.Domain.Validation;

namespace Something.Domain.Models.Weather;

public sealed record WeatherCurrent(
    double Temperature,
    double ApparentTemperature,
    int WeatherCode,
    double WindSpeed)
{
    public string Icon => WeatherRules.GetCodePresentation(WeatherCode).Icon;
    public string Condition => WeatherRules.GetCodePresentation(WeatherCode).Label;
}
