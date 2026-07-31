using System.Diagnostics;
using System.Runtime.InteropServices;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.FileSystem;
using Something.Domain.Exceptions;
using Something.Domain.Models.FileExplorer;

namespace Something.Infrastructure.FileExplorer;

public sealed class FileExplorerService(ILogger<FileExplorerService> logger) : IFileExplorerService
{
    private const int DefaultPageSize = 250;
    private const int MaximumPageSize = 500;
    private const int MaximumEntries = 2000;
    private const int MaximumQueryLength = 255;
    private readonly object _rootLock = new();
    private readonly Dictionary<string, ExplorerRoot> _roots = new(StringComparer.Ordinal);
    private readonly Dictionary<string, string> _rootIdsByPath = new(StringComparer.OrdinalIgnoreCase);
    private bool _standardRootsRegistered;

    public Task<IReadOnlyList<FileExplorerPlace>> GetPlacesAsync(CancellationToken cancellationToken = default)
    {
        return Task.Run<IReadOnlyList<FileExplorerPlace>>(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                EnsureStandardRoots();
                lock (_rootLock)
                {
                    return _roots.Values
                        .OrderBy(root => PlaceOrder(root.Kind))
                        .ThenBy(root => root.Label, StringComparer.CurrentCultureIgnoreCase)
                        .Select(root => new FileExplorerPlace(root.Id, root.Label, root.Kind))
                        .ToArray();
                }
            },
            cancellationToken);
    }

    public Task<FileExplorerListing> RegisterChosenRootAsync(
        string folderPath,
        CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                var root = RegisterRoot(folderPath, RootLabel(folderPath), "custom");
                return ListDirectory(root, string.Empty, string.Empty, 0, DefaultPageSize, cancellationToken);
            },
            cancellationToken);
    }

    public Task<FileExplorerListing> ListDirectoryAsync(
        string rootId,
        string relativePath,
        string query,
        int offset,
        int limit,
        CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                var root = GetRoot(rootId);
                return ListDirectory(root, relativePath, query, offset, limit, cancellationToken);
            },
            cancellationToken);
    }

    public Task OpenFileAsync(
        string rootId,
        string relativePath,
        CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                var target = ResolveRegularFile(GetRoot(rootId), relativePath);
                try
                {
                    Process.Start(new ProcessStartInfo(target) { UseShellExecute = true });
                }
                catch (Exception exception)
                {
                    logger.LogWarning(exception, "Windows could not open an explorer file.");
                    throw new InvalidOperationException("That file could not be opened.");
                }
            },
            cancellationToken);
    }

    public Task RecycleFileAsync(
        string rootId,
        string relativePath,
        CancellationToken cancellationToken = default)
    {
        return Task.Run(
            () =>
            {
                cancellationToken.ThrowIfCancellationRequested();
                var target = ResolveRegularFile(GetRoot(rootId), relativePath);
                if (!OperatingSystem.IsWindows())
                {
                    throw new PlatformNotSupportedException("Recycle Bin deletion is available only on Windows.");
                }

                var operation = new ShellFileOperation
                {
                    Function = FileOperationDelete,
                    From = target + '\0',
                    Flags = FileOperationAllowUndo | FileOperationNoConfirmation |
                            FileOperationNoErrorUi | FileOperationSilent,
                };
                var result = SHFileOperation(ref operation);
                if (result != 0 || operation.AnyOperationsAborted)
                {
                    logger.LogWarning("Recycle Bin operation failed with shell code {ShellCode}.", result);
                    throw new InvalidOperationException("That file could not be moved to the Recycle Bin.");
                }
            },
            cancellationToken);
    }

    private FileExplorerListing ListDirectory(
        ExplorerRoot root,
        string relativePath,
        string query,
        int offset,
        int limit,
        CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        if (offset < 0 || offset >= MaximumEntries)
        {
            throw new ValidationException("Choose a valid directory page.");
        }

        query = (query ?? string.Empty).Trim();
        if (query.EnumerateRunes().Count() > MaximumQueryLength)
        {
            throw new ValidationException("Search text is too long.");
        }

        limit = limit <= 0 ? DefaultPageSize : Math.Min(limit, MaximumPageSize);
        limit = Math.Min(limit, MaximumEntries - offset);
        var (directory, normalizedPath) = ResolveDirectory(root, relativePath);

        try
        {
            var candidates = directory.EnumerateFileSystemInfos().Select(info => CreateEntry(normalizedPath, info));
            var truncated = false;
            IReadOnlyList<FileExplorerEntry> page;
            var hasMore = false;
            int nextOffset;
            if (query.Length > 0)
            {
                var matches = new List<FileExplorerEntry>();
                foreach (var candidate in candidates)
                {
                    cancellationToken.ThrowIfCancellationRequested();
                    if (!candidate.Name.Contains(query, StringComparison.CurrentCultureIgnoreCase))
                    {
                        continue;
                    }

                    if (matches.Count == MaximumEntries)
                    {
                        truncated = true;
                        break;
                    }

                    matches.Add(candidate);
                }

                matches.Sort(EntryComparer.Instance);
                var start = Math.Min(offset, matches.Count);
                page = matches.Skip(start).Take(limit).ToArray();
                nextOffset = start + page.Count;
                hasMore = nextOffset < matches.Count;
            }
            else
            {
                var rawPage = candidates.Skip(offset).Take(limit + 1).ToArray();
                hasMore = rawPage.Length > limit && offset + limit < MaximumEntries;
                truncated = rawPage.Length > limit && !hasMore;
                page = rawPage.Take(limit).Order(EntryComparer.Instance).ToArray();
                nextOffset = offset + page.Count;
            }

            return new FileExplorerListing(
                root.Id,
                root.Label,
                normalizedPath,
                query,
                ParentPath(normalizedPath),
                normalizedPath.Length > 0,
                Breadcrumbs(root.Label, normalizedPath),
                page,
                nextOffset,
                hasMore,
                truncated,
                MaximumEntries);
        }
        catch (UnauthorizedAccessException exception)
        {
            logger.LogInformation("Explorer directory access was denied ({ExceptionType}).", exception.GetType().Name);
            throw new InvalidOperationException("You do not have permission to browse that folder.");
        }
        catch (DirectoryNotFoundException exception)
        {
            logger.LogInformation("Explorer directory disappeared ({ExceptionType}).", exception.GetType().Name);
            throw new InvalidOperationException("That folder is no longer available.");
        }
        catch (IOException exception)
        {
            logger.LogInformation("Explorer directory could not be read ({ExceptionType}).", exception.GetType().Name);
            throw new InvalidOperationException("That folder could not be read.");
        }
    }

    private void EnsureStandardRoots()
    {
        lock (_rootLock)
        {
            if (_standardRootsRegistered)
            {
                return;
            }

            _standardRootsRegistered = true;
        }

        var profile = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        if (profile.Length > 0)
        {
            var driveRoot = Path.GetPathRoot(profile);
            if (!string.IsNullOrWhiteSpace(driveRoot) && Directory.Exists(driveRoot))
            {
                RegisterRoot(driveRoot, $"Home ({driveRoot.TrimEnd(Path.DirectorySeparatorChar)})", "home");
            }
        }

        RegisterSpecialFolder(Environment.SpecialFolder.DesktopDirectory, "Desktop", "desktop");
        RegisterSpecialFolder(Environment.SpecialFolder.MyDocuments, "Documents", "documents");
        var downloads = Path.Combine(profile, "Downloads");
        if (Directory.Exists(downloads)) RegisterRoot(downloads, "Downloads", "downloads");

        if (OperatingSystem.IsWindows() && Directory.Exists(@"D:\"))
        {
            RegisterRoot(@"D:\", "Home 2", "home-secondary");
        }
    }

    private void RegisterSpecialFolder(Environment.SpecialFolder folder, string label, string kind)
    {
        var path = Environment.GetFolderPath(folder);
        if (path.Length > 0 && Directory.Exists(path)) RegisterRoot(path, label, kind);
    }

    private ExplorerRoot RegisterRoot(string path, string label, string kind)
    {
        var canonical = CanonicalRoot(path);
        lock (_rootLock)
        {
            if (_rootIdsByPath.TryGetValue(canonical, out var existingId))
            {
                return _roots[existingId];
            }

            var root = new ExplorerRoot(Guid.NewGuid().ToString("N"), label, kind, canonical);
            _roots.Add(root.Id, root);
            _rootIdsByPath.Add(canonical, root.Id);
            return root;
        }
    }

    private ExplorerRoot GetRoot(string rootId)
    {
        rootId = (rootId ?? string.Empty).Trim();
        lock (_rootLock)
        {
            return _roots.TryGetValue(rootId, out var root)
                ? root
                : throw new ValidationException("Choose a valid explorer location.");
        }
    }

    private static string CanonicalRoot(string path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            throw new ValidationException("Choose a folder to browse.");
        }

        var directory = new DirectoryInfo(Path.GetFullPath(path.Trim()));
        if (!directory.Exists)
        {
            throw new ValidationException("That folder is no longer available.");
        }

        if (directory.Attributes.HasFlag(FileAttributes.ReparsePoint))
        {
            directory = directory.ResolveLinkTarget(returnFinalTarget: true) as DirectoryInfo
                ?? throw new ValidationException("The selected folder link could not be resolved.");
        }

        return Path.TrimEndingDirectorySeparator(directory.FullName);
    }

    private static (DirectoryInfo Directory, string NormalizedPath) ResolveDirectory(
        ExplorerRoot root,
        string requestedPath)
    {
        var normalized = NormalizeRelativePath(requestedPath, allowRoot: true);
        var current = new DirectoryInfo(root.Path);
        if (normalized.Length > 0)
        {
            foreach (var segment in normalized.Split('/'))
            {
                current = new DirectoryInfo(Path.Combine(current.FullName, segment));
                if (!current.Exists)
                {
                    throw new InvalidOperationException("That folder is no longer available.");
                }

                if (current.Attributes.HasFlag(FileAttributes.ReparsePoint))
                {
                    throw new ValidationException("Folder links cannot be browsed from this location.");
                }
            }
        }

        EnsureWithinRoot(root.Path, current.FullName, "That folder is outside the selected location.");
        return (current, normalized);
    }

    private static string ResolveRegularFile(ExplorerRoot root, string requestedPath)
    {
        var normalized = NormalizeRelativePath(requestedPath, allowRoot: false);
        var parentPath = ParentPath(normalized);
        var (parent, _) = ResolveDirectory(root, parentPath);
        var file = new FileInfo(Path.Combine(parent.FullName, Path.GetFileName(normalized)));
        EnsureWithinRoot(root.Path, file.FullName, "That file is outside the selected location.");
        if (Directory.Exists(file.FullName))
        {
            throw new ValidationException("Choose a file, not a folder.");
        }

        if (!file.Exists)
        {
            throw new InvalidOperationException("That file is no longer available.");
        }

        if (file.Attributes.HasFlag(FileAttributes.ReparsePoint))
        {
            throw new ValidationException("File links cannot be opened or deleted.");
        }

        return file.FullName;
    }

    private static string NormalizeRelativePath(string requestedPath, bool allowRoot)
    {
        requestedPath = (requestedPath ?? string.Empty).Trim();
        if (requestedPath.Length == 0)
        {
            if (allowRoot) return string.Empty;
            throw new ValidationException("Choose a file inside this location.");
        }

        requestedPath = requestedPath.Replace('/', Path.DirectorySeparatorChar);
        if (Path.IsPathRooted(requestedPath))
        {
            throw new ValidationException("Choose an item inside this location.");
        }

        var anchor = Path.Combine(Path.GetPathRoot(Environment.CurrentDirectory)!, "__something_explorer_anchor__");
        var combined = Path.GetFullPath(Path.Combine(anchor, requestedPath));
        var relative = Path.GetRelativePath(anchor, combined);
        if (relative == ".." || relative.StartsWith(".." + Path.DirectorySeparatorChar, StringComparison.Ordinal))
        {
            throw new ValidationException("Choose an item inside this location.");
        }

        return relative == "." ? string.Empty : relative.Replace(Path.DirectorySeparatorChar, '/');
    }

    private static void EnsureWithinRoot(string root, string candidate, string message)
    {
        var relative = Path.GetRelativePath(root, Path.GetFullPath(candidate));
        if (Path.IsPathRooted(relative) || relative == ".." ||
            relative.StartsWith(".." + Path.DirectorySeparatorChar, StringComparison.Ordinal))
        {
            throw new ValidationException(message);
        }
    }

    private static FileExplorerEntry CreateEntry(string parentPath, FileSystemInfo info)
    {
        var isLink = info.Attributes.HasFlag(FileAttributes.ReparsePoint);
        var isDirectory = info.Attributes.HasFlag(FileAttributes.Directory) && !isLink;
        var size = info is FileInfo file && !isLink ? file.Length : 0;
        var extension = Path.GetExtension(info.Name).TrimStart('.');
        var type = isLink ? "Link" : isDirectory ? "Folder" : extension.Length == 0 ? "File" : $"{extension.ToUpperInvariant()} file";
        return new FileExplorerEntry(
            info.Name,
            parentPath.Length == 0 ? info.Name : $"{parentPath}/{info.Name}",
            type,
            size,
            info.LastWriteTimeUtc == DateTime.MinValue ? null : new DateTimeOffset(info.LastWriteTimeUtc, TimeSpan.Zero),
            isDirectory,
            isLink);
    }

    private static IReadOnlyList<FileExplorerBreadcrumb> Breadcrumbs(string rootLabel, string relativePath)
    {
        var result = new List<FileExplorerBreadcrumb> { new(rootLabel, string.Empty) };
        var current = string.Empty;
        foreach (var part in relativePath.Split('/', StringSplitOptions.RemoveEmptyEntries))
        {
            current = current.Length == 0 ? part : $"{current}/{part}";
            result.Add(new FileExplorerBreadcrumb(part, current));
        }

        return result;
    }

    private static string ParentPath(string relativePath)
    {
        var separator = relativePath.LastIndexOf('/');
        return separator < 0 ? string.Empty : relativePath[..separator];
    }

    private static string RootLabel(string path)
    {
        var full = Path.TrimEndingDirectorySeparator(Path.GetFullPath(path));
        return Path.GetFileName(full) is { Length: > 0 } name ? name : full;
    }

    private static int PlaceOrder(string kind) => kind switch
    {
        "home" => 0,
        "home-secondary" => 1,
        "desktop" => 2,
        "documents" => 3,
        "downloads" => 4,
        _ => 5,
    };

    private sealed record ExplorerRoot(string Id, string Label, string Kind, string Path);

    private sealed class EntryComparer : IComparer<FileExplorerEntry>
    {
        public static EntryComparer Instance { get; } = new();

        public int Compare(FileExplorerEntry? left, FileExplorerEntry? right)
        {
            if (ReferenceEquals(left, right)) return 0;
            if (left is null) return -1;
            if (right is null) return 1;
            if (left.IsDirectory != right.IsDirectory) return left.IsDirectory ? -1 : 1;
            return StringComparer.CurrentCultureIgnoreCase.Compare(left.Name, right.Name);
        }
    }

    private const uint FileOperationDelete = 3;
    private const ushort FileOperationSilent = 0x0004;
    private const ushort FileOperationNoConfirmation = 0x0010;
    private const ushort FileOperationAllowUndo = 0x0040;
    private const ushort FileOperationNoErrorUi = 0x0400;

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct ShellFileOperation
    {
        public nint Window;
        public uint Function;
        [MarshalAs(UnmanagedType.LPWStr)] public string From;
        [MarshalAs(UnmanagedType.LPWStr)] public string? To;
        public ushort Flags;
        [MarshalAs(UnmanagedType.Bool)] public bool AnyOperationsAborted;
        public nint NameMappings;
        [MarshalAs(UnmanagedType.LPWStr)] public string? ProgressTitle;
    }

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern int SHFileOperation(ref ShellFileOperation operation);
}
