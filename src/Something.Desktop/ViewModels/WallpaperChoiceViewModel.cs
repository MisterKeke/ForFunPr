using System.Windows.Media;
using CommunityToolkit.Mvvm.ComponentModel;
using Something.Domain.Models.Wallpapers;

namespace Something.Desktop.ViewModels;

public sealed partial class WallpaperChoiceViewModel(
    WallpaperItem item,
    ImageSource? preview,
    bool isSelected) : ObservableObject
{
    public WallpaperItem Item { get; } = item;
    public string Key => Item.Key;
    public string Id => Item.Id;
    public string Label => Item.Label;
    public string Description => Item.Description;
    public bool CanDelete => !Item.IsBuiltIn;
    public ImageSource? Preview { get; } = preview;
    public bool HasPreview => Preview is not null;

    [ObservableProperty]
    private bool _isSelected = isSelected;
}
