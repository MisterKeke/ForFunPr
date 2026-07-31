using Something.Domain.Enums;

namespace Something.Application.Messages;

public sealed record FavoriteCategoriesChangedMessage(FavoriteSource Source, DateTimeOffset ChangedAt);
