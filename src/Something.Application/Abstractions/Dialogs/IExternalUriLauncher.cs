namespace Something.Application.Abstractions.Dialogs;

public interface IExternalUriLauncher
{
    Task OpenAsync(string uri, CancellationToken cancellationToken = default);
}
