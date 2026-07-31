using System.Buffers.Binary;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Extensions.Logging.Abstractions;
using Microsoft.Win32.SafeHandles;
using Something.Domain.Exceptions;
using Something.Infrastructure.FileExplorer;

namespace Something.Infrastructure.Tests;

public sealed class FileExplorerTests
{
    [Fact]
    public async Task RegisteredRootUsesRelativePathsAndSupportsSearchAndPagination()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var root = CreateTemporaryDirectory();
        try
        {
            Directory.CreateDirectory(Path.Combine(root, "inside"));
            await File.WriteAllTextAsync(Path.Combine(root, "alpha.txt"), "a", cancellationToken);
            await File.WriteAllTextAsync(Path.Combine(root, "inside", "nested.txt"), "b", cancellationToken);
            var service = new FileExplorerService(NullLogger<FileExplorerService>.Instance);

            var listing = await service.RegisterChosenRootAsync(root, cancellationToken);
            Assert.All(listing.Entries, entry => Assert.False(Path.IsPathRooted(entry.Path)));
            Assert.Contains(listing.Entries, entry => entry.Path == "inside" && entry.IsDirectory);
            Assert.Contains(listing.Entries, entry => entry.Path == "alpha.txt" && !entry.IsDirectory);

            var nested = await service.ListDirectoryAsync(
                listing.RootId, "inside", string.Empty, 0, 1, cancellationToken);
            Assert.Equal("inside/nested.txt", Assert.Single(nested.Entries).Path);
            Assert.Equal("inside", nested.Breadcrumbs[^1].Path);

            var search = await service.ListDirectoryAsync(
                listing.RootId, string.Empty, "ALPHA", 0, 250, cancellationToken);
            Assert.Equal("alpha.txt", Assert.Single(search.Entries).Path);
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task TraversalUnknownRootsAndDirectoryFileActionsAreRejected()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var root = CreateTemporaryDirectory();
        try
        {
            Directory.CreateDirectory(Path.Combine(root, "folder"));
            var service = new FileExplorerService(NullLogger<FileExplorerService>.Instance);
            var listing = await service.RegisterChosenRootAsync(root, cancellationToken);

            await Assert.ThrowsAsync<ValidationException>(() => service.ListDirectoryAsync(
                listing.RootId, "../outside", string.Empty, 0, 250, cancellationToken));
            await Assert.ThrowsAsync<ValidationException>(() => service.ListDirectoryAsync(
                "not-a-root", string.Empty, string.Empty, 0, 250, cancellationToken));
            await Assert.ThrowsAsync<ValidationException>(() => service.OpenFileAsync(
                listing.RootId, "folder", cancellationToken));
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task ReparsePointCannotBeUsedToEscapeARegisteredRoot()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        var root = CreateTemporaryDirectory();
        var outside = CreateTemporaryDirectory();
        var link = Path.Combine(root, "outside-link");
        try
        {
            CreateDirectoryJunction(link, outside);

            var service = new FileExplorerService(NullLogger<FileExplorerService>.Instance);
            var listing = await service.RegisterChosenRootAsync(root, cancellationToken);
            var entry = Assert.Single(listing.Entries);
            Assert.True(entry.IsSymbolicLink);
            Assert.False(entry.IsDirectory);
            await Assert.ThrowsAsync<ValidationException>(() => service.ListDirectoryAsync(
                listing.RootId, entry.Path, string.Empty, 0, 250, cancellationToken));
        }
        finally
        {
            if (Directory.Exists(link)) Directory.Delete(link);
            Directory.Delete(root, recursive: true);
            Directory.Delete(outside, recursive: true);
        }
    }

    private static string CreateTemporaryDirectory()
    {
        var path = Path.Combine(Path.GetTempPath(), "SomethingCSharp.ExplorerTests", Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(path);
        return path;
    }

    private static void CreateDirectoryJunction(string junctionPath, string targetPath)
    {
        Directory.CreateDirectory(junctionPath);
        var substituteName = @"\??\" + Path.GetFullPath(targetPath).TrimEnd(Path.DirectorySeparatorChar);
        var printName = Path.GetFullPath(targetPath).TrimEnd(Path.DirectorySeparatorChar);
        var substituteBytes = Encoding.Unicode.GetBytes(substituteName);
        var printBytes = Encoding.Unicode.GetBytes(printName);
        var pathBuffer = Encoding.Unicode.GetBytes(substituteName + '\0' + printName + '\0');
        var buffer = new byte[16 + pathBuffer.Length];
        BinaryPrimitives.WriteUInt32LittleEndian(buffer.AsSpan(0, 4), 0xA0000003);
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.AsSpan(4, 2), checked((ushort)(8 + pathBuffer.Length)));
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.AsSpan(8, 2), 0);
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.AsSpan(10, 2), checked((ushort)substituteBytes.Length));
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.AsSpan(12, 2), checked((ushort)(substituteBytes.Length + 2)));
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.AsSpan(14, 2), checked((ushort)printBytes.Length));
        pathBuffer.CopyTo(buffer, 16);

        using var handle = CreateFile(
            junctionPath,
            0x40000000,
            0x00000001 | 0x00000002 | 0x00000004,
            0,
            3,
            0x00200000 | 0x02000000,
            0);
        if (handle.IsInvalid)
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }

        if (!DeviceIoControl(
            handle, 0x000900A4, buffer, buffer.Length,
            null, 0, out _, 0))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern SafeFileHandle CreateFile(
        string fileName,
        uint desiredAccess,
        uint shareMode,
        nint securityAttributes,
        uint creationDisposition,
        uint flagsAndAttributes,
        nint templateFile);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool DeviceIoControl(
        SafeFileHandle device,
        uint controlCode,
        byte[] input,
        int inputSize,
        byte[]? output,
        int outputSize,
        out int bytesReturned,
        nint overlapped);
}
