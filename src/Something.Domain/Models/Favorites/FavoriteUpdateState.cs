namespace Something.Domain.Models.Favorites;

public sealed record FavoriteUpdateWindow(
    DateTimeOffset? PublishedAfter,
    DateTimeOffset? PublishedUntil);

public sealed record FavoriteUpdateWindows(
    FavoriteUpdateWindow NewWhileClosed,
    FavoriteUpdateWindow NewWhileOpen);

public sealed record FavoriteUpdateState(
    DateTimeOffset? PreviousOpenedAt,
    DateTimeOffset? CurrentOpenedAt,
    DateTimeOffset? PreviousRefreshAt,
    DateTimeOffset? LastRefreshAt)
{
    public FavoriteUpdateWindows UpdateWindows => new(
        new FavoriteUpdateWindow(PreviousOpenedAt, CurrentOpenedAt),
        new FavoriteUpdateWindow(PreviousRefreshAt, LastRefreshAt));
}
