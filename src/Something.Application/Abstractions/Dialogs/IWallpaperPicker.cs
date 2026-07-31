namespace Something.Application.Abstractions.Dialogs;

public interface IWallpaperPicker
{
    Task<string?> PickWallpaperAsync(CancellationToken cancellationToken = default);
}
