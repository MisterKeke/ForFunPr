using Something.Domain.Models.Favorites;

namespace Something.Desktop.ViewModels;

public sealed record FavoriteChannelViewModel(
    FavoriteChannel Channel,
    string CategoryName)
{
    public string SourceId => Channel.SourceId;
    public string DisplayName => Channel.DisplayName;
    public string CategoryLabel => CategoryName.Length == 0 ? "Uncategorized" : CategoryName;
}
