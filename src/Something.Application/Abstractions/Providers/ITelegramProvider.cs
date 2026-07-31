using Something.Domain.Models.Posts;

namespace Something.Application.Abstractions.Providers;

public interface ITelegramProvider
{
    Task<IReadOnlyList<TelegramPost>> GetPostsAsync(
        string username,
        int before = 0,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default);
}
