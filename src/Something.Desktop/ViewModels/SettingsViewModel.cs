using System.Runtime.InteropServices;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Wallpapers;
using Something.Desktop.Navigation;

namespace Something.Desktop.ViewModels;

public sealed partial class SettingsViewModel(
    IWallpaperService wallpapers,
    IMessenger messenger,
    ILogger<SettingsViewModel> logger) : PageViewModel
{
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.Settings;
    public override string Title => "Settings";
    public override string Subtitle => "Appearance and information for this native desktop build.";

    public string ThemeLabel => "Dark neon";
    public string ApplicationVersion =>
        typeof(SettingsViewModel).Assembly.GetName().Version?.ToString(3) ?? "Development";
    public string RuntimeLabel => RuntimeInformation.FrameworkDescription;
    public string PlatformLabel => RuntimeInformation.OSDescription;
    public string DataProfileLabel => "Something.CSharp.Dev (isolated development profile)";

    [ObservableProperty]
    private string _selectedWallpaperLabel = "Loading…";

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            var settings = await wallpapers.GetAsync(_pageCancellation.Token);
            SelectedWallpaperLabel = settings.Wallpapers
                .FirstOrDefault(item => item.Key == settings.Selected)?.Label ?? "Hu Tao";
            IsEmpty = false;
        }
        catch (OperationCanceledException) when (_pageCancellation.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            logger.LogError(exception, "Settings could not read appearance information.");
            ErrorMessage = "Appearance settings could not be loaded.";
        }
        finally
        {
            IsBusy = false;
        }
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }

    [RelayCommand]
    private void OpenWallpapers() =>
        messenger.Send(new NavigateRequestedMessage(PageKey.Wallpapers));
}
