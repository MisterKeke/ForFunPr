using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Persistence;

namespace Something.Infrastructure.Database;

public sealed class DatabaseInitializer(
    IApplicationPaths applicationPaths,
    IDatabaseConnectionFactory connectionFactory,
    IEnumerable<IMigration> migrations,
    ILogger<DatabaseInitializer> logger) : IDatabaseInitializer, IDisposable
{
    private readonly SemaphoreSlim _initializationLock = new(1, 1);
    private bool _initialized;
    private bool _disposed;

    public async Task InitializeAsync(CancellationToken cancellationToken = default)
    {
        ObjectDisposedException.ThrowIf(_disposed, this);
        await _initializationLock.WaitAsync(cancellationToken);

        try
        {
            if (_initialized)
            {
                return;
            }

            await applicationPaths.EnsureDirectoriesAsync(cancellationToken);
            await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);

            await CreateMigrationTableAsync(connection, cancellationToken);
            var appliedVersions = await ReadAppliedVersionsAsync(connection, cancellationToken);
            var orderedMigrations = migrations.OrderBy(migration => migration.Version).ToArray();
            EnsureMigrationDefinitionsAreValid(orderedMigrations);

            foreach (var migration in orderedMigrations)
            {
                if (appliedVersions.Contains(migration.Version))
                {
                    continue;
                }

                await ApplyMigrationAsync(connection, migration, cancellationToken);
                logger.LogInformation(
                    "Applied database migration {MigrationVersion}: {MigrationName}",
                    migration.Version,
                    migration.Name);
            }

            _initialized = true;
        }
        finally
        {
            _initializationLock.Release();
        }
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _initializationLock.Dispose();
        _disposed = true;
    }

    private static async Task CreateMigrationTableAsync(
        SqliteConnection connection,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.CommandText = """
            CREATE TABLE IF NOT EXISTS schema_migrations (
                version INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            );
            """;
        await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static async Task<HashSet<int>> ReadAppliedVersionsAsync(
        SqliteConnection connection,
        CancellationToken cancellationToken)
    {
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT version FROM schema_migrations;";
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);

        var versions = new HashSet<int>();
        while (await reader.ReadAsync(cancellationToken))
        {
            versions.Add(reader.GetInt32(0));
        }

        return versions;
    }

    private static void EnsureMigrationDefinitionsAreValid(
        IReadOnlyCollection<IMigration> orderedMigrations)
    {
        var duplicateVersion = orderedMigrations
            .GroupBy(migration => migration.Version)
            .FirstOrDefault(group => group.Count() > 1);

        if (duplicateVersion is not null)
        {
            throw new InvalidOperationException(
                $"Migration version {duplicateVersion.Key} is registered more than once.");
        }
    }

    private static async Task ApplyMigrationAsync(
        SqliteConnection connection,
        IMigration migration,
        CancellationToken cancellationToken)
    {
        await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(
            cancellationToken);

        try
        {
            await migration.UpAsync(connection, transaction, cancellationToken);

            await using var command = connection.CreateCommand();
            command.Transaction = transaction;
            command.CommandText = """
                INSERT INTO schema_migrations (version, name)
                VALUES ($version, $name);
                """;
            command.Parameters.AddWithValue("$version", migration.Version);
            command.Parameters.AddWithValue("$name", migration.Name);
            await command.ExecuteNonQueryAsync(cancellationToken);

            await transaction.CommitAsync(cancellationToken);
        }
        catch
        {
            try
            {
                await transaction.RollbackAsync(CancellationToken.None);
            }
            catch
            {
                // Preserve the original migration failure.
            }

            throw;
        }
    }
}
