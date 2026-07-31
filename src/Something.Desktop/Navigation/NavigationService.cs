namespace Something.Desktop.Navigation;

public sealed class NavigationService : INavigationService, IDisposable
{
    private readonly IReadOnlyDictionary<PageKey, INavigationPage> _pages;
    private readonly SemaphoreSlim _navigationLock = new(1, 1);
    private CancellationTokenSource? _pageCancellation;
    private bool _disposed;

    public NavigationService(IEnumerable<INavigationPage> pages)
    {
        _pages = pages.ToDictionary(page => page.PageKey);
    }

    public INavigationPage? CurrentPage { get; private set; }

    public event EventHandler<NavigationChangedEventArgs>? CurrentPageChanged;

    public async Task NavigateAsync(
        PageKey pageKey,
        CancellationToken cancellationToken = default)
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        await _navigationLock.WaitAsync(cancellationToken);

        try
        {
            if (CurrentPage?.PageKey == pageKey)
            {
                return;
            }

            if (!_pages.TryGetValue(pageKey, out var nextPage))
            {
                throw new InvalidOperationException($"Page {pageKey} is not registered.");
            }

            _pageCancellation?.Cancel();
            _pageCancellation?.Dispose();
            CurrentPage?.OnNavigatedFrom();

            _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            CurrentPage = nextPage;
            CurrentPageChanged?.Invoke(this, new NavigationChangedEventArgs(nextPage));

            await nextPage.OnNavigatedToAsync(_pageCancellation.Token);
        }
        finally
        {
            _navigationLock.Release();
        }
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _navigationLock.Dispose();
        _disposed = true;
    }
}
