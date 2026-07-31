using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Features.Wallpapers;
using Something.Desktop.Navigation;
using Something.Desktop.Services;
using Something.Domain.Exceptions;
using Something.Domain.Models.Wallpapers;

namespace Something.Desktop.ViewModels;

public sealed partial class WallpapersViewModel(
    IWallpaperService wallpapers,
    IWallpaperPicker picker,
    IWallpaperImageLoader imageLoader,
    IConfirmationDialog confirmation,
    ILogger<WallpapersViewModel> logger) : PageViewModel
{
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.Wallpapers;
    public override string Title => "Wallpapers";
    public override string Subtitle => "Choose a bundled background or import an application-owned image.";

    public ObservableCollection<WallpaperChoiceViewModel> Choices { get; } = [];

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        CancelPage();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await RunAsync(wallpapers.GetAsync, "Wallpapers could not be loaded.", _pageCancellation.Token);
    }

    public override void OnNavigatedFrom() => CancelPage();

    [RelayCommand]
    private Task SelectAsync(WallpaperChoiceViewModel choice, CancellationToken cancellationToken) =>
        RunAsync(
            token => wallpapers.SelectAsync(choice.Key, token),
            "The wallpaper could not be selected.",
            cancellationToken);

    [RelayCommand]
    private async Task ImportAsync(CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        try
        {
            var path = await picker.PickWallpaperAsync(linked.Token);
            if (path is null) return;
            await RunAsync(
                token => wallpapers.ImportAsync(path, token),
                "The wallpaper could not be imported.",
                linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
    }

    [RelayCommand]
    private Task DeleteAsync(WallpaperChoiceViewModel choice, CancellationToken cancellationToken)
    {
        if (!choice.CanDelete ||
            !confirmation.Confirm($"Delete \"{choice.Label}\" from this device?", "Something"))
        {
            return Task.CompletedTask;
        }

        return RunAsync(
            token => wallpapers.DeleteAsync(choice.Id, token),
            "The wallpaper could not be deleted.",
            cancellationToken);
    }

    private async Task RunAsync(
        Func<CancellationToken, Task<WallpaperSettings>> operation,
        string fallback,
        CancellationToken cancellationToken)
    {
        using var linked = CreateLinkedToken(cancellationToken);
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            Apply(await operation(linked.Token));
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            logger.LogError(exception, "Wallpaper operation failed.");
            ErrorMessage = exception is ValidationException or NotFoundException
                ? exception.Message
                : fallback;
        }
        finally
        {
            IsBusy = false;
        }
    }

    private void Apply(WallpaperSettings settings)
    {
        Choices.Clear();
        foreach (var item in settings.Wallpapers)
        {
            try
            {
                Choices.Add(new WallpaperChoiceViewModel(
                    item, imageLoader.Load(item), item.Key == settings.Selected));
            }
            catch (ValidationException exception)
            {
                logger.LogWarning(exception, "A wallpaper preview could not be decoded.");
            }
        }

        IsEmpty = Choices.Count == 0;
    }

    private CancellationTokenSource CreateLinkedToken(CancellationToken cancellationToken) =>
        _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);

    private void CancelPage()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }
}
