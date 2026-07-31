using System.IO;
using System.Windows;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Something.Application.Abstractions.FileSystem;
using Something.Domain.Exceptions;
using Something.Domain.Models.Wallpapers;

namespace Something.Desktop.Services;

public sealed class WallpaperImageService : IWallpaperImageLoader, IWallpaperImageValidator
{
    public Task ValidateAsync(
        string filePath,
        string mimeType,
        CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                _ = DecodeFile(filePath, mimeType);
                cancellationToken.ThrowIfCancellationRequested();
            },
            cancellationToken);
    }

    public ImageSource? Load(WallpaperItem wallpaper)
    {
        if (wallpaper.Key == "builtin:original") return null;
        if (!wallpaper.IsBuiltIn) return DecodeFile(wallpaper.FilePath, wallpaper.MimeType);

        var filename = wallpaper.Id switch
        {
            "sandrone" => "sandrone.jpg",
            "hu-tao" => "hu-tao.jpg",
            "skirk" => "skirk.jpg",
            _ => throw new ValidationException("The bundled wallpaper is not available."),
        };
        var uri = new Uri(
            $"pack://application:,,,/Something.Desktop;component/Assets/Wallpapers/{filename}",
            UriKind.Absolute);
        var resource = System.Windows.Application.GetResourceStream(uri)
            ?? throw new ValidationException("The bundled wallpaper is not available.");
        using (resource.Stream)
        {
            return Decode(resource.Stream, "image/jpeg");
        }
    }

    private static ImageSource DecodeFile(string path, string mimeType)
    {
        try
        {
            using var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read);
            return Decode(stream, mimeType);
        }
        catch (Exception exception) when (exception is IOException or NotSupportedException or FileFormatException)
        {
            throw new ValidationException(
                mimeType == "image/webp"
                    ? "This WebP image cannot be decoded. Install the Windows WebP Image Extension or choose JPEG/PNG."
                    : "The selected wallpaper image could not be decoded.");
        }
    }

    private static ImageSource Decode(Stream stream, string mimeType)
    {
        BitmapDecoder decoder = mimeType switch
        {
            "image/jpeg" => new JpegBitmapDecoder(
                stream, BitmapCreateOptions.PreservePixelFormat, BitmapCacheOption.OnLoad),
            "image/png" => new PngBitmapDecoder(
                stream, BitmapCreateOptions.PreservePixelFormat, BitmapCacheOption.OnLoad),
            // WebP is deliberately routed through the installed WIC WebP codec.
            "image/webp" => BitmapDecoder.Create(
                stream, BitmapCreateOptions.PreservePixelFormat, BitmapCacheOption.OnLoad),
            _ => throw new ValidationException("Choose a JPEG, PNG, or WebP image."),
        };
        var frame = decoder.Frames.FirstOrDefault()
            ?? throw new ValidationException("The selected wallpaper has no image frame.");
        frame.Freeze();
        return frame;
    }
}
