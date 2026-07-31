using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using System.Net.Http.Headers;
using Something.Application.Abstractions.Persistence;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;
using Something.Application.Abstractions.Providers;
using Something.Infrastructure.Providers.Currency;
using Something.Infrastructure.Providers.Weather;
using Something.Infrastructure.Providers.Telegram;
using Something.Infrastructure.Providers.YouTube;
using Something.Application.Abstractions.FileSystem;
using Something.Infrastructure.FileExplorer;

namespace Something.Infrastructure;

public static class DependencyInjection
{
    public static IServiceCollection AddSomethingInfrastructure(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(services);
        ArgumentNullException.ThrowIfNull(configuration);

        services.AddSingleton<IApplicationPaths, ApplicationPaths>();
        services.AddSingleton<IDatabaseConnectionFactory, SqliteConnectionFactory>();
        services.AddSingleton<IDatabaseInitializer, DatabaseInitializer>();
        services.AddSingleton<DatabaseWriteCoordinator>();
        services.AddSingleton<ITaskRepository, TaskRepository>();
        services.AddSingleton<INoteRepository, NoteRepository>();
        services.AddSingleton<IBookmarkRepository, BookmarkRepository>();
        services.AddSingleton<ICurrencyFavoriteRepository, CurrencyFavoriteRepository>();
        services.AddSingleton<ICurrencyProvider, FrankfurterProvider>();
        services.AddSingleton<IWeatherCacheRepository, WeatherCacheRepository>();
        services.AddSingleton<IWeatherProvider, OpenMeteoProvider>();
        services.AddSingleton<IFavoriteCategoryRepository, FavoriteCategoryRepository>();
        services.AddSingleton<IFavoriteChannelRepository, FavoriteChannelRepository>();
        services.AddSingleton<IFavoriteUpdateRepository, FavoriteUpdateRepository>();
        services.AddSingleton<ITelegramProvider, TelegramProvider>();
        services.AddSingleton<IYouTubeProvider, YouTubeProvider>();
        services.AddSingleton<IFileExplorerService, FileExplorerService>();
        services.AddSingleton<IWallpaperRepository, WallpaperRepository>();

        services.AddHttpClient(FrankfurterProvider.ClientName, client =>
        {
            client.BaseAddress = new Uri("https://api.frankfurter.dev/");
            client.Timeout = TimeSpan.FromSeconds(15);
            client.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
            client.DefaultRequestHeaders.UserAgent.ParseAdd("Something.CSharp/1.0");
            client.MaxResponseContentBufferSize = 1024 * 1024;
        }).ConfigurePrimaryHttpMessageHandler(static () => new SocketsHttpHandler
        {
            AllowAutoRedirect = false,
            MaxConnectionsPerServer = 4,
            ConnectTimeout = TimeSpan.FromSeconds(15),
        });
        services.AddHttpClient(OpenMeteoProvider.ClientName, client =>
        {
            client.Timeout = TimeSpan.FromSeconds(15);
            client.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
            client.DefaultRequestHeaders.UserAgent.ParseAdd("Something.CSharp/1.0");
            client.MaxResponseContentBufferSize = 2 * 1024 * 1024;
        }).ConfigurePrimaryHttpMessageHandler(static () => new SocketsHttpHandler
        {
            AllowAutoRedirect = false,
            MaxConnectionsPerServer = 4,
            ConnectTimeout = TimeSpan.FromSeconds(15),
        });
        services.AddHttpClient(TelegramProvider.ClientName, client =>
        {
            client.Timeout = TimeSpan.FromSeconds(15);
            client.DefaultRequestHeaders.UserAgent.ParseAdd(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Something.CSharp/1.0");
            client.DefaultRequestHeaders.AcceptLanguage.ParseAdd("en-US,en;q=0.5");
            client.MaxResponseContentBufferSize = 2 * 1024 * 1024;
        }).ConfigurePrimaryHttpMessageHandler(static () => new SocketsHttpHandler
        {
            AllowAutoRedirect = false,
            MaxConnectionsPerServer = 4,
            ConnectTimeout = TimeSpan.FromSeconds(15),
        });
        services.AddHttpClient(YouTubeProvider.ClientName, client =>
        {
            client.Timeout = TimeSpan.FromSeconds(15);
            client.DefaultRequestHeaders.UserAgent.ParseAdd(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Something.CSharp/1.0");
            client.DefaultRequestHeaders.AcceptLanguage.ParseAdd("en-US,en;q=0.9");
            client.MaxResponseContentBufferSize = 2 * 1024 * 1024;
        }).ConfigurePrimaryHttpMessageHandler(static () => new SocketsHttpHandler
        {
            AllowAutoRedirect = false,
            MaxConnectionsPerServer = 4,
            ConnectTimeout = TimeSpan.FromSeconds(15),
        });

        services.AddSingleton<IMigration, Migration001CoreSchema>();
        services.AddSingleton<IMigration, Migration002FavoriteSchema>();
        services.AddSingleton<IMigration, Migration003ApplicationState>();
        services.AddSingleton<IMigration, Migration004FavoriteUpdates>();
        services.AddSingleton<IMigration, Migration005WeatherLocation>();
        services.AddSingleton<IMigration, Migration006WeatherCache>();
        services.AddSingleton<IMigration, Migration007FavoriteNews>();
        services.AddSingleton<IMigration, Migration008TaskMetadata>();
        services.AddSingleton<IMigration, Migration009Wallpapers>();
        services.AddSingleton<IMigration, Migration010Notes>();
        services.AddSingleton<IMigration, Migration011Bookmarks>();

        return services;
    }
}
