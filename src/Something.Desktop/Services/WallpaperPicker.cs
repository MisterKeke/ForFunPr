using Microsoft.Win32;
using Something.Application.Abstractions.Dialogs;

namespace Something.Desktop.Services;

public sealed class WallpaperPicker : IWallpaperPicker
{
    public Task<string?> PickWallpaperAsync(CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var dialog = new OpenFileDialog
        {
            Title = "Choose a wallpaper",
            Filter = "Images (*.jpg;*.jpeg;*.png;*.webp)|*.jpg;*.jpeg;*.png;*.webp",
            CheckFileExists = true,
            Multiselect = false,
        };
        var accepted = dialog.ShowDialog() == true;
        cancellationToken.ThrowIfCancellationRequested();
        return Task.FromResult(accepted ? dialog.FileName : null);
    }
}
