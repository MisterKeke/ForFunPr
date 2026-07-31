namespace Something.Application.Abstractions.FileSystem;

public interface IWallpaperImageValidator
{
    Task ValidateAsync(
        string filePath,
        string mimeType,
        CancellationToken cancellationToken = default);
}
