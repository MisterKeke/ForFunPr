namespace Something.Application.Messages;

public sealed record FavoritesChangedMessage(string Source, DateTimeOffset ChangedAt);
