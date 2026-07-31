using Something.Domain.Models.Posts;

namespace Something.Application.Abstractions.Providers;

public interface IYouTubeProvider
{
    Task<(string ChannelId, string Handle)> ResolveChannelAsync(
        string reference,
        CancellationToken cancellationToken = default);
    Task<IReadOnlyList<YouTubeVideo>> GetVideosAsync(
        string reference,
        bool forceRefresh = false,
        CancellationToken cancellationToken = default);
}
