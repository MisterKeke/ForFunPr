using System.IO;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Channels;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Persistence;

namespace Something.Desktop.Services;

public sealed partial class FileLogProvider(IApplicationPaths paths) : ILoggerProvider, IHostedService
{
    private readonly Channel<string> _entries = Channel.CreateBounded<string>(new BoundedChannelOptions(1024)
    {
        SingleReader = true,
        SingleWriter = false,
        FullMode = BoundedChannelFullMode.DropOldest,
    });
    private readonly CancellationTokenSource _stopping = new();
    private Task? _writerTask;

    public ILogger CreateLogger(string categoryName) => new FileLogger(this, categoryName);

    public async Task StartAsync(CancellationToken cancellationToken)
    {
        await paths.EnsureDirectoriesAsync(cancellationToken);
        _writerTask = WriteEntriesAsync(_stopping.Token);
    }

    public async Task StopAsync(CancellationToken cancellationToken)
    {
        _entries.Writer.TryComplete();
        if (_writerTask is not null)
        {
            await _writerTask.WaitAsync(cancellationToken);
        }
    }

    public void Dispose()
    {
        _entries.Writer.TryComplete();
        _stopping.Cancel();
        _stopping.Dispose();
    }

    private void Enqueue(
        LogLevel level,
        string category,
        string message,
        Exception? exception)
    {
        var builder = new StringBuilder(512)
            .Append(DateTimeOffset.UtcNow.ToString("O"))
            .Append(' ').Append(level.ToString().ToUpperInvariant())
            .Append(' ').Append(category)
            .Append(" - ").Append(message);
        if (exception is not null)
        {
            builder.AppendLine().Append(exception);
        }

        var sanitized = Sanitize(builder.ToString());
        if (sanitized.Length > 16 * 1024)
        {
            sanitized = sanitized[..(16 * 1024)] + " [truncated]";
        }

        _entries.Writer.TryWrite(sanitized);
    }

    private async Task WriteEntriesAsync(CancellationToken cancellationToken)
    {
        var filename = $"something-{DateTime.UtcNow:yyyyMMdd}.log";
        var logPath = Path.Combine(paths.LogsDirectory, filename);
        await using var stream = new FileStream(
            logPath, FileMode.Append, FileAccess.Write, FileShare.Read,
            16 * 1024, FileOptions.Asynchronous);
        await using var writer = new StreamWriter(stream, new UTF8Encoding(false)) { AutoFlush = true };
        await foreach (var entry in _entries.Reader.ReadAllAsync(cancellationToken))
        {
            await writer.WriteLineAsync(entry);
        }
    }

    private string Sanitize(string value)
    {
        value = value.Replace(paths.DataDirectory, "[app-data]", StringComparison.OrdinalIgnoreCase);
        value = UrlQueryRegex().Replace(value, "$1?[redacted]");
        return WindowsPathRegex().Replace(value, "[local-path]");
    }

    [GeneratedRegex(@"(https://[^\s?]+)\?[^\s]+", RegexOptions.IgnoreCase | RegexOptions.CultureInvariant)]
    private static partial Regex UrlQueryRegex();

    [GeneratedRegex(@"\b[a-z]:\\[^\r\n]*", RegexOptions.IgnoreCase | RegexOptions.CultureInvariant)]
    private static partial Regex WindowsPathRegex();

    private sealed class FileLogger(FileLogProvider provider, string category) : ILogger
    {
        public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;
        public bool IsEnabled(LogLevel logLevel) => logLevel >= LogLevel.Information;

        public void Log<TState>(
            LogLevel logLevel,
            EventId eventId,
            TState state,
            Exception? exception,
            Func<TState, Exception?, string> formatter)
        {
            _ = eventId;
            if (IsEnabled(logLevel))
            {
                provider.Enqueue(logLevel, category, formatter(state, exception), exception);
            }
        }
    }
}
