using CommunityToolkit.Mvvm.Messaging.Messages;

namespace Something.Application.Messages;

public sealed class TasksChangedMessage(DateTimeOffset changedAt)
    : ValueChangedMessage<DateTimeOffset>(changedAt);
