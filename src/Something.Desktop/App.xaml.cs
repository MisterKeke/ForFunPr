using System.Diagnostics;
using System.Windows;
using System.Windows.Threading;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Something.Application;
using Something.Application.Abstractions.Persistence;
using Something.Application.Features.Favorites;
using Something.Desktop.ViewModels;
using Something.Infrastructure;

namespace Something.Desktop;

public partial class App : System.Windows.Application
{
    private readonly CancellationTokenSource _applicationCancellation = new();
    private IHost? _host;

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        DispatcherUnhandledException += OnDispatcherUnhandledException;
        TaskScheduler.UnobservedTaskException += OnUnobservedTaskException;

        try
        {
            var builder = Host.CreateApplicationBuilder(e.Args);
            builder.Services.AddSomethingApplication();
            builder.Services.AddSomethingInfrastructure(builder.Configuration);
            builder.Services.AddSomethingDesktop();

            _host = builder.Build();
            await _host.StartAsync(_applicationCancellation.Token);

            var databaseInitializer = _host.Services.GetRequiredService<IDatabaseInitializer>();
            await databaseInitializer.InitializeAsync(_applicationCancellation.Token);

            var favoriteUpdates = _host.Services.GetRequiredService<IFavoriteUpdateService>();
            await favoriteUpdates.RecordApplicationOpenAsync(_applicationCancellation.Token);

            var mainViewModel = _host.Services.GetRequiredService<MainViewModel>();
            await mainViewModel.InitializeAsync(_applicationCancellation.Token);

            var mainWindow = _host.Services.GetRequiredService<MainWindow>();
            MainWindow = mainWindow;
            mainWindow.Show();
        }
        catch (Exception exception)
        {
            _applicationCancellation.Cancel();
            LogCritical(exception, "Application startup failed.");
            await StopHostAsync();

            MessageBox.Show(
                "Something could not start its local services. Please restart the application.",
                "Something",
                MessageBoxButton.OK,
                MessageBoxImage.Error);

            Shutdown(-1);
        }
    }

    protected override async void OnExit(ExitEventArgs e)
    {
        _applicationCancellation.Cancel();
        await StopHostAsync();

        DispatcherUnhandledException -= OnDispatcherUnhandledException;
        TaskScheduler.UnobservedTaskException -= OnUnobservedTaskException;
        _applicationCancellation.Dispose();

        base.OnExit(e);
    }

    private async Task StopHostAsync()
    {
        var host = Interlocked.Exchange(ref _host, null);
        if (host is null)
        {
            return;
        }

        var logger = host.Services.GetService<ILogger<App>>();

        try
        {
            using var shutdownTimeout = new CancellationTokenSource(TimeSpan.FromSeconds(10));
            await host.StopAsync(shutdownTimeout.Token);
        }
        catch (Exception exception)
        {
            Trace.TraceError("Application shutdown failed: {0}", exception);
            logger?.LogError(exception, "Application shutdown failed.");
        }
        finally
        {
            host.Dispose();
        }
    }

    private void OnDispatcherUnhandledException(
        object sender,
        DispatcherUnhandledExceptionEventArgs e)
    {
        LogCritical(e.Exception, "An unhandled dispatcher exception occurred.");
        e.Handled = true;

        MessageBox.Show(
            "Something encountered an unexpected error.",
            "Something",
            MessageBoxButton.OK,
            MessageBoxImage.Error);
    }

    private void OnUnobservedTaskException(
        object? sender,
        UnobservedTaskExceptionEventArgs e)
    {
        LogCritical(e.Exception, "An unobserved background task exception occurred.");
        e.SetObserved();
    }

    private void LogCritical(Exception exception, string message)
    {
        Trace.TraceError("{0} {1}", message, exception);
        _host?.Services.GetService<ILogger<App>>()?.LogCritical(exception, "{Message}", message);
    }
}
