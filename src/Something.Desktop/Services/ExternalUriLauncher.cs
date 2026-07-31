using System.Diagnostics;
using Something.Application.Abstractions.Dialogs;
using Something.Domain.Validation;

namespace Something.Desktop.Services;

public sealed class ExternalUriLauncher : IExternalUriLauncher
{
    public Task OpenAsync(string uri, CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var (safeUri, _) = BookmarkRules.NormalizeUrl(uri);
        Process.Start(new ProcessStartInfo(safeUri) { UseShellExecute = true });
        return Task.CompletedTask;
    }
}
