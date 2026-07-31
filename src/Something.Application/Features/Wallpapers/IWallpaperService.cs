using Something.Domain.Models.Wallpapers;

namespace Something.Application.Features.Wallpapers;

public interface IWallpaperService
{
    Task<WallpaperSettings> GetAsync(CancellationToken cancellationToken = default);
    Task<WallpaperSettings> SelectAsync(string selection, CancellationToken cancellationToken = default);
    Task<WallpaperSettings> ImportAsync(string sourcePath, CancellationToken cancellationToken = default);
    Task<WallpaperSettings> DeleteAsync(string id, CancellationToken cancellationToken = default);
}
