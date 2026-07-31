namespace Something.Application.Messages;

public sealed record WallpaperChangedMessage(string Selection, DateTimeOffset ChangedAt);
