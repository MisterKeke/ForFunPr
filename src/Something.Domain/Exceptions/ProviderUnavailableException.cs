namespace Something.Domain.Exceptions;

public sealed class ProviderUnavailableException : Exception
{
    public ProviderUnavailableException(string message)
        : base(message)
    {
    }

    public ProviderUnavailableException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
