using Microsoft.Extensions.DependencyInjection;
using Something.Application.Abstractions.Dialogs;
using Something.Desktop.Navigation;
using Something.Desktop.Services;
using Something.Desktop.ViewModels;
using Something.Application.Abstractions.FileSystem;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace Something.Desktop;

public static class DependencyInjection
{
    public static IServiceCollection AddSomethingDesktop(this IServiceCollection services)
    {
        ArgumentNullException.ThrowIfNull(services);

        services.AddSingleton<IExternalUriLauncher, ExternalUriLauncher>();
        services.AddSingleton<IFolderPicker, FolderPicker>();
        services.AddSingleton<IConfirmationDialog, ConfirmationDialog>();
        services.AddSingleton<WallpaperImageService>();
        services.AddSingleton<IWallpaperImageLoader>(provider => provider.GetRequiredService<WallpaperImageService>());
        services.AddSingleton<IWallpaperImageValidator>(provider => provider.GetRequiredService<WallpaperImageService>());
        services.AddSingleton<IWallpaperPicker, WallpaperPicker>();
        services.AddSingleton<FileLogProvider>();
        services.AddSingleton<ILoggerProvider>(provider => provider.GetRequiredService<FileLogProvider>());
        services.AddSingleton<IHostedService>(provider => provider.GetRequiredService<FileLogProvider>());

        services.AddSingleton<DashboardViewModel>();
        services.AddSingleton<TasksViewModel>();
        services.AddSingleton<NotesViewModel>();
        services.AddSingleton<BookmarksViewModel>();
        services.AddSingleton<TelegramViewModel>();
        services.AddSingleton<YouTubeViewModel>();
        services.AddSingleton<WeatherViewModel>();
        services.AddSingleton<CurrenciesViewModel>();
        services.AddSingleton<FileExplorerViewModel>();
        services.AddSingleton<WallpapersViewModel>();
        services.AddSingleton<SettingsViewModel>();

        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<DashboardViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<TasksViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<NotesViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<BookmarksViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<TelegramViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<YouTubeViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<WeatherViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<CurrenciesViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<FileExplorerViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<WallpapersViewModel>());
        services.AddSingleton<INavigationPage>(provider => provider.GetRequiredService<SettingsViewModel>());

        services.AddSingleton<INavigationService, NavigationService>();
        services.AddSingleton<MainViewModel>();
        services.AddSingleton<MainWindow>();

        return services;
    }
}
