namespace Something.Application.Abstractions.Persistence;

public interface ICurrencyFavoriteRepository
{
    Task<bool> AddAsync(string baseCode, string targetCode, CancellationToken cancellationToken = default);
    Task<bool> RemoveAsync(string baseCode, string targetCode, CancellationToken cancellationToken = default);
    Task<IReadOnlyList<string>> ListAsync(CancellationToken cancellationToken = default);
}
