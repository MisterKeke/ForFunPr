using CommunityToolkit.Mvvm.ComponentModel;

namespace Something.Desktop.ViewModels;

public sealed partial class TaskSubtaskDraftViewModel(
    long id,
    string title,
    bool isCompleted) : ObservableObject
{
    public long Id { get; } = id;

    [ObservableProperty]
    private string _title = title;

    [ObservableProperty]
    private bool _isCompleted = isCompleted;
}
