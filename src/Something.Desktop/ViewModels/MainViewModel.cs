using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using CommunityToolkit.Mvvm.Messaging;
using Something.Desktop.Navigation;
using Something.Application.Features.Wallpapers;
using Something.Application.Messages;
using Something.Desktop.Services;
using System.Windows.Media;

namespace Something.Desktop.ViewModels;

public sealed partial class MainViewModel : ObservableObject, IDisposable
{
    private readonly INavigationService _navigationService;
    private readonly IMessenger _messenger;
    private readonly IWallpaperService _wallpaperService;
    private readonly IWallpaperImageLoader _wallpaperImageLoader;
    private bool _disposed;

    public MainViewModel(
        INavigationService navigationService,
        IMessenger messenger,
        IWallpaperService wallpaperService,
        IWallpaperImageLoader wallpaperImageLoader)
    {
        _navigationService = navigationService;
        _messenger = messenger;
        _wallpaperService = wallpaperService;
        _wallpaperImageLoader = wallpaperImageLoader;
        _navigationService.CurrentPageChanged += OnCurrentPageChanged;
        _messenger.Register<MainViewModel, NavigateRequestedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveNavigationRequest(message));
        _messenger.Register<MainViewModel, WallpaperChangedMessage>(
            this,
            static (recipient, message) => recipient.ReceiveWallpaperChanged(message));

        NavigationItems =
        [
            new(PageKey.Dashboard, "Dashboard", "\uE80F"),
            new(PageKey.Tasks, "Tasks", "\uE823"),
            new(PageKey.Notes, "Notes", "\uE70B"),
            new(PageKey.Bookmarks, "Bookmarks", "\uE734"),
            new(PageKey.Telegram, "Telegram", "\uE715"),
            new(PageKey.YouTube, "YouTube", "\uE714"),
            new(PageKey.Weather, "Weather", "\uE706"),
            new(PageKey.Currencies, "Currencies", "\uE8C7"),
            new(PageKey.FileExplorer, "File Explorer", "\uEC50"),
            new(PageKey.Wallpapers, "Wallpapers", "\uE91B"),
            new(PageKey.Settings, "Settings", "\uE713"),
        ];
    }

    public ObservableCollection<NavigationItem> NavigationItems { get; }

    public ObservableCollection<string> Notifications { get; } = [];

    [ObservableProperty]
    private INavigationPage? _currentPage;

    [ObservableProperty]
    private PageKey _selectedPage;

    [ObservableProperty]
    private bool _isBusy;

    [ObservableProperty]
    private object? _activeDialog;

    [ObservableProperty]
    private ImageSource? _wallpaperImage;

    public async Task InitializeAsync(CancellationToken cancellationToken = default)
    {
        await LoadWallpaperAsync(cancellationToken);
        await NavigateAsync(PageKey.Dashboard, cancellationToken);
    }

    [RelayCommand]
    private async Task NavigateAsync(
        PageKey pageKey,
        CancellationToken cancellationToken = default)
    {
        IsBusy = true;

        try
        {
            await _navigationService.NavigateAsync(pageKey, cancellationToken);
        }
        finally
        {
            IsBusy = false;
        }
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _navigationService.CurrentPageChanged -= OnCurrentPageChanged;
        _messenger.UnregisterAll(this);
        _disposed = true;
    }

    private void OnCurrentPageChanged(object? sender, NavigationChangedEventArgs e)
    {
        CurrentPage = e.Page;
        SelectedPage = e.Page.PageKey;

        foreach (var item in NavigationItems)
        {
            item.IsSelected = item.PageKey == SelectedPage;
        }
    }

    private void ReceiveNavigationRequest(NavigateRequestedMessage message)
    {
        _ = NavigateFromMessageAsync(message.PageKey);
    }

    private async Task NavigateFromMessageAsync(PageKey pageKey)
    {
        try
        {
            await NavigateAsync(pageKey);
        }
        catch (Exception exception)
        {
            Notifications.Add("That page could not be opened.");
            System.Diagnostics.Trace.TraceError("Dashboard navigation failed: {0}", exception);
        }
    }

    private void ReceiveWallpaperChanged(WallpaperChangedMessage message)
    {
        _ = message;
        _ = ReloadWallpaperFromMessageAsync();
    }

    private async Task ReloadWallpaperFromMessageAsync()
    {
        try
        {
            await LoadWallpaperAsync(CancellationToken.None);
        }
        catch (Exception exception)
        {
            System.Diagnostics.Trace.TraceError("Wallpaper refresh failed: {0}", exception);
        }
    }

    private async Task LoadWallpaperAsync(CancellationToken cancellationToken)
    {
        var settings = await _wallpaperService.GetAsync(cancellationToken);
        var selected = settings.Wallpapers.FirstOrDefault(item => item.Key == settings.Selected);
        WallpaperImage = selected is null ? null : _wallpaperImageLoader.Load(selected);
    }
}
