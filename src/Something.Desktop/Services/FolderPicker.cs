using Microsoft.Win32;
using Something.Application.Abstractions.Dialogs;

namespace Something.Desktop.Services;

public sealed class FolderPicker : IFolderPicker
{
    public Task<string?> PickFolderAsync(CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var dialog = new OpenFolderDialog
        {
            Title = "Choose a folder to browse",
            Multiselect = false,
        };
        var accepted = dialog.ShowDialog() == true;
        cancellationToken.ThrowIfCancellationRequested();
        return Task.FromResult(accepted ? dialog.FolderName : null);
    }
}
