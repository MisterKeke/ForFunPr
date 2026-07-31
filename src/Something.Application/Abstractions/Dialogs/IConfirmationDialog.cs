namespace Something.Application.Abstractions.Dialogs;

public interface IConfirmationDialog
{
    bool Confirm(string message, string title);
}
