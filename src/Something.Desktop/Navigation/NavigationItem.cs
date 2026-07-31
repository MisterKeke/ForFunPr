using CommunityToolkit.Mvvm.ComponentModel;

namespace Something.Desktop.Navigation;

public sealed partial class NavigationItem(
    PageKey pageKey,
    string label,
    string glyph) : ObservableObject
{
    public PageKey PageKey { get; } = pageKey;

    public string Label { get; } = label;

    public string Glyph { get; } = glyph;

    [ObservableProperty]
    private bool _isSelected;
}
