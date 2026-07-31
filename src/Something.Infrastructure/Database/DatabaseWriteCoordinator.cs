namespace Something.Infrastructure.Database;

public sealed class DatabaseWriteCoordinator : IDisposable
{
    private readonly SemaphoreSlim _writeLock = new(1, 1);
    private bool _disposed;

    public async Task<T> ExecuteAsync<T>(
        Func<CancellationToken, Task<T>> operation,
        CancellationToken cancellationToken = default)
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        ArgumentNullException.ThrowIfNull(operation);

        await _writeLock.WaitAsync(cancellationToken);
        try
        {
            return await operation(cancellationToken);
        }
        finally
        {
            _writeLock.Release();
        }
    }

    public Task ExecuteAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(operation);
        return ExecuteAsync(
            async token =>
            {
                await operation(token);
                return true;
            },
            cancellationToken);
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _writeLock.Dispose();
        _disposed = true;
    }
}
