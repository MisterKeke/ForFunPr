namespace Something.Desktop.Navigation;

public interface INavigationPage
{
    PageKey PageKey { get; }

    Task OnNavigatedToAsync(CancellationToken cancellationToken);

    void OnNavigatedFrom();
}
