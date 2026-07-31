namespace Something.Desktop.Navigation;

public interface INavigationService
{
    INavigationPage? CurrentPage { get; }

    event EventHandler<NavigationChangedEventArgs>? CurrentPageChanged;

    Task NavigateAsync(PageKey pageKey, CancellationToken cancellationToken = default);
}
