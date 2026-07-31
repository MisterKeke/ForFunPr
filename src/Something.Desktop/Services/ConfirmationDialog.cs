using System.Windows;
using Something.Application.Abstractions.Dialogs;

namespace Something.Desktop.Services;

public sealed class ConfirmationDialog : IConfirmationDialog
{
    public bool Confirm(string message, string title) =>
        MessageBox.Show(
            message,
            title,
            MessageBoxButton.YesNo,
            MessageBoxImage.Warning,
            MessageBoxResult.No) == MessageBoxResult.Yes;
}
