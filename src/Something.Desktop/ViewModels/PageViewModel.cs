using CommunityToolkit.Mvvm.ComponentModel;
using Something.Desktop.Navigation;

namespace Something.Desktop.ViewModels;

public abstract partial class PageViewModel : ObservableObject, INavigationPage
{
    protected PageViewModel()
    {
        IsEmpty = true;
    }

    public abstract PageKey PageKey { get; }

    public abstract string Title { get; }

    public abstract string Subtitle { get; }

    public bool HasError => !string.IsNullOrWhiteSpace(ErrorMessage);

    [ObservableProperty]
    private bool _isBusy;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasError))]
    private string? _errorMessage;

    [ObservableProperty]
    private bool _isEmpty;

    public virtual Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        return Task.CompletedTask;
    }

    public virtual void OnNavigatedFrom()
    {
    }
}
