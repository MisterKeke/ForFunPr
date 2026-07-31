using Microsoft.Extensions.DependencyInjection;
using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Features.Bookmarks;
using Something.Application.Features.Currencies;
using Something.Application.Features.Favorites;
using Something.Application.Features.Notes;
using Something.Application.Features.Posts;
using Something.Application.Features.Tasks;
using Something.Application.Features.Weather;
using Something.Application.Features.Wallpapers;

namespace Something.Application;

public static class DependencyInjection
{
    public static IServiceCollection AddSomethingApplication(this IServiceCollection services)
    {
        ArgumentNullException.ThrowIfNull(services);

        services.AddSingleton(TimeProvider.System);
        services.AddSingleton<IMessenger, WeakReferenceMessenger>();
        services.AddSingleton<ITaskService, TaskService>();
        services.AddSingleton<INoteService, NoteService>();
        services.AddSingleton<IBookmarkService, BookmarkService>();
        services.AddSingleton<ICurrencyService, CurrencyService>();
        services.AddSingleton<IWeatherService, WeatherService>();
        services.AddSingleton<IFavoriteCategoryService, FavoriteCategoryService>();
        services.AddSingleton<IFavoriteUpdateService, FavoriteUpdateService>();
        services.AddSingleton<ITelegramService, TelegramService>();
        services.AddSingleton<IYouTubeService, YouTubeService>();
        services.AddSingleton<IWallpaperService, WallpaperService>();

        return services;
    }
}
