namespace Something.Domain.Models.Weather;

public sealed record WeatherForecast(
    WeatherCurrent Current,
    IReadOnlyList<WeatherDay> Days,
    string Timezone);
