namespace Something.Domain.Models.Wallpapers;

public sealed record WallpaperItem(
    string Key,
    string Id,
    string Label,
    string Description,
    bool IsBuiltIn,
    string FilePath,
    string MimeType,
    long ByteSize);

public sealed record WallpaperSettings(
    string Selected,
    bool SelectionSaved,
    IReadOnlyList<WallpaperItem> Wallpapers);
