using CommunityToolkit.Mvvm.Messaging;
using Something.Application.Abstractions.Persistence;
using Something.Application.Messages;
using Something.Domain.Exceptions;
using Something.Domain.Models.Wallpapers;

namespace Something.Application.Features.Wallpapers;

public sealed class WallpaperService(
    IWallpaperRepository repository,
    IMessenger messenger) : IWallpaperService
{
    public Task<WallpaperSettings> GetAsync(CancellationToken cancellationToken = default) =>
        repository.GetAsync(cancellationToken);

    public async Task<WallpaperSettings> SelectAsync(
        string selection,
        CancellationToken cancellationToken = default)
    {
        selection = (selection ?? string.Empty).Trim();
        if (selection.Length == 0)
        {
            throw new ValidationException("Choose a valid wallpaper.");
        }

        var settings = await repository.SelectAsync(selection, cancellationToken);
        Publish(settings.Selected);
        return settings;
    }

    public async Task<WallpaperSettings> ImportAsync(
        string sourcePath,
        CancellationToken cancellationToken = default)
    {
        if (string.IsNullOrWhiteSpace(sourcePath))
        {
            throw new ValidationException("Choose a wallpaper image.");
        }

        var settings = await repository.ImportAsync(sourcePath, cancellationToken);
        Publish(settings.Selected);
        return settings;
    }

    public async Task<WallpaperSettings> DeleteAsync(
        string id,
        CancellationToken cancellationToken = default)
    {
        id = (id ?? string.Empty).Trim();
        var settings = await repository.DeleteAsync(id, cancellationToken);
        Publish(settings.Selected);
        return settings;
    }

    private void Publish(string selection) =>
        messenger.Send(new WallpaperChangedMessage(selection, DateTimeOffset.UtcNow));
}
