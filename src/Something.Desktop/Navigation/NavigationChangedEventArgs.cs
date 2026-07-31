namespace Something.Desktop.Navigation;

public sealed class NavigationChangedEventArgs(INavigationPage page) : EventArgs
{
    public INavigationPage Page { get; } = page;
}
