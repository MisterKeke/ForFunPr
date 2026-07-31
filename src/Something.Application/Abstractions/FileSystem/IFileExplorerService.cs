using Something.Domain.Models.FileExplorer;

namespace Something.Application.Abstractions.FileSystem;

public interface IFileExplorerService
{
    Task<IReadOnlyList<FileExplorerPlace>> GetPlacesAsync(CancellationToken cancellationToken = default);

    Task<FileExplorerListing> RegisterChosenRootAsync(
        string folderPath,
        CancellationToken cancellationToken = default);

    Task<FileExplorerListing> ListDirectoryAsync(
        string rootId,
        string relativePath,
        string query,
        int offset,
        int limit,
        CancellationToken cancellationToken = default);

    Task OpenFileAsync(
        string rootId,
        string relativePath,
        CancellationToken cancellationToken = default);

    Task RecycleFileAsync(
        string rootId,
        string relativePath,
        CancellationToken cancellationToken = default);
}
