namespace Something.Application.Abstractions.Dialogs;

public interface IFolderPicker
{
    Task<string?> PickFolderAsync(CancellationToken cancellationToken = default);
}
