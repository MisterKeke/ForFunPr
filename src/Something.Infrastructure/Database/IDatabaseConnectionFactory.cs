using Microsoft.Data.Sqlite;

namespace Something.Infrastructure.Database;

public interface IDatabaseConnectionFactory
{
    ValueTask<SqliteConnection> OpenConnectionAsync(
        CancellationToken cancellationToken = default);
}
