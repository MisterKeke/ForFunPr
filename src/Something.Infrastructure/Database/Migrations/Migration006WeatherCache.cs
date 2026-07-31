using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database.Migrations;

public sealed class Migration006WeatherCache : IMigration
{
    public int Version => 6;

    public string Name => "cache saved weather forecasts";

    public async Task UpAsync(
        SqliteConnection connection,
        SqliteTransaction transaction,
        CancellationToken cancellationToken = default)
    {
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "location",
            "forecast_json",
            "ALTER TABLE location ADD COLUMN forecast_json TEXT;",
            cancellationToken);
        await MigrationSql.AddColumnIfMissingAsync(
            connection,
            transaction,
            "location",
            "forecast_updated_at",
            "ALTER TABLE location ADD COLUMN forecast_updated_at TEXT;",
            cancellationToken);
    }
}
