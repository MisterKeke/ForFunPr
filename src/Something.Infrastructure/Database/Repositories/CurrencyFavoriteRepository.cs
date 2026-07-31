using Something.Application.Abstractions.Persistence;

namespace Something.Infrastructure.Database.Repositories;

public sealed class CurrencyFavoriteRepository(
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator) : ICurrencyFavoriteRepository
{
    public Task<bool> AddAsync(
        string baseCode,
        string targetCode,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    INSERT OR IGNORE INTO favorite_rates (base, quote)
                    VALUES ($base, $quote);
                    """;
                command.Parameters.AddWithValue("$base", baseCode);
                command.Parameters.AddWithValue("$quote", targetCode);
                return await command.ExecuteNonQueryAsync(token) == 1;
            },
            cancellationToken);
    }

    public Task<bool> RemoveAsync(
        string baseCode,
        string targetCode,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await using var command = connection.CreateCommand();
                command.CommandText = """
                    DELETE FROM favorite_rates
                    WHERE base = $base AND quote = $quote;
                    """;
                command.Parameters.AddWithValue("$base", baseCode);
                command.Parameters.AddWithValue("$quote", targetCode);
                return await command.ExecuteNonQueryAsync(token) == 1;
            },
            cancellationToken);
    }

    public async Task<IReadOnlyList<string>> ListAsync(CancellationToken cancellationToken = default)
    {
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT base, quote FROM favorite_rates ORDER BY base, quote;";
        var pairs = new List<string>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            pairs.Add($"{reader.GetString(0)}:{reader.GetString(1)}");
        }

        return pairs;
    }
}
