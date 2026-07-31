namespace Something.Domain.Exceptions;

public sealed class ApplicationNotReadyException : Exception
{
    public ApplicationNotReadyException(string message)
        : base(message)
    {
    }

    public ApplicationNotReadyException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
