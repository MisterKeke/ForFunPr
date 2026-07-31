using CommunityToolkit.Mvvm.ComponentModel;
using Something.Domain.Enums;
using Something.Domain.Models.Favorites;

namespace Something.Desktop.ViewModels;

public sealed partial class FavoriteCategoryDraftViewModel : ObservableObject
{
    public FavoriteCategoryDraftViewModel(FavoriteCategory category)
    {
        Id = category.Id;
        Name = category.Name;
        Source = category.Source;
    }

    public long Id { get; }
    public FavoriteSource Source { get; }

    [ObservableProperty]
    private string _name;
}
