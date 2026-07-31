using System.Windows.Media;
using Something.Domain.Models.Wallpapers;

namespace Something.Desktop.Services;

public interface IWallpaperImageLoader
{
    ImageSource? Load(WallpaperItem wallpaper);
}
