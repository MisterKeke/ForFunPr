namespace Something.Domain.Models.FileExplorer;

public sealed record FileExplorerPlace(string RootId, string Label, string Kind)
{
    public string Glyph => Kind switch
    {
        "home" or "home-secondary" => "",
        "desktop" => "",
        "documents" => "",
        "downloads" => "",
        _ => "",
    };
}

public sealed record FileExplorerBreadcrumb(string Label, string Path);

public sealed record FileExplorerEntry(
    string Name,
    string Path,
    string Type,
    long Size,
    DateTimeOffset? ModifiedAt,
    bool IsDirectory,
    bool IsSymbolicLink)
{
    public string SizeLabel => IsDirectory ? "—" : FormatSize(Size);
    public string ModifiedLabel => ModifiedAt?.LocalDateTime.ToString("g") ?? "—";
    public string Glyph => IsSymbolicLink ? "" : IsDirectory ? "" : "";
    public bool CanActOnFile => !IsDirectory && !IsSymbolicLink;

    private static string FormatSize(long bytes)
    {
        if (bytes < 1024) return $"{bytes} B";
        string[] units = ["KB", "MB", "GB", "TB"];
        var value = (double)bytes;
        var index = -1;
        do
        {
            value /= 1024;
            index++;
        }
        while (value >= 1024 && index < units.Length - 1);

        return $"{value:0.#} {units[index]}";
    }
}

public sealed record FileExplorerListing(
    string RootId,
    string RootLabel,
    string Path,
    string Query,
    string ParentPath,
    bool CanGoUp,
    IReadOnlyList<FileExplorerBreadcrumb> Breadcrumbs,
    IReadOnlyList<FileExplorerEntry> Entries,
    int NextOffset,
    bool HasMore,
    bool Truncated,
    int MaximumItems);
