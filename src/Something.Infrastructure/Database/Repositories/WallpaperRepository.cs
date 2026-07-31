using System.Globalization;
using System.Security.Cryptography;
using System.Text.RegularExpressions;
using Microsoft.Data.Sqlite;
using Something.Application.Abstractions.FileSystem;
using Something.Application.Abstractions.Persistence;
using Something.Domain.Exceptions;
using Something.Domain.Models.Wallpapers;

namespace Something.Infrastructure.Database.Repositories;

public sealed partial class WallpaperRepository(
    IApplicationPaths paths,
    IDatabaseConnectionFactory connectionFactory,
    DatabaseWriteCoordinator writeCoordinator,
    IWallpaperImageValidator imageValidator) : IWallpaperRepository
{
    public const string DefaultSelection = "builtin:hu-tao";
    private const string SelectionStateKey = "selected_wallpaper";
    private const long MaximumBytes = 20L * 1024 * 1024;
    private static readonly IReadOnlyList<WallpaperItem> BuiltIns =
    [
        new("builtin:original", "original", "Original", "Midnight glow", true, "", "", 0),
        new("builtin:sandrone", "sandrone", "Sandrone", "Warm crimson", true, "", "image/jpeg", 0),
        new("builtin:hu-tao", "hu-tao", "Hu Tao", "Dusky violet", true, "", "image/jpeg", 0),
        new("builtin:skirk", "skirk", "Skirk", "Deep ocean", true, "", "image/jpeg", 0),
    ];

    public async Task<WallpaperSettings> GetAsync(CancellationToken cancellationToken = default)
    {
        var users = await ListUsersAsync(cancellationToken);
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT value FROM app_state WHERE key = $key;";
        command.Parameters.AddWithValue("$key", SelectionStateKey);
        var value = await command.ExecuteScalarAsync(cancellationToken);
        var requested = value as string;
        var saved = requested is not null && SelectionExists(requested, users);
        return new WallpaperSettings(
            saved ? requested! : DefaultSelection,
            saved,
            [.. BuiltIns, .. users]);
    }

    public Task<WallpaperSettings> SelectAsync(
        string selection,
        CancellationToken cancellationToken = default)
    {
        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                var users = await ListUsersAsync(token);
                if (!SelectionExists(selection, users))
                {
                    throw new ValidationException("Choose a valid wallpaper.");
                }

                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                await UpsertSelectionAsync(connection, null, selection, token);
                return new WallpaperSettings(selection, true, [.. BuiltIns, .. users]);
            },
            cancellationToken);
    }

    public async Task<WallpaperSettings> ImportAsync(
        string sourcePath,
        CancellationToken cancellationToken = default)
    {
        var source = new FileInfo(Path.GetFullPath(sourcePath));
        if (!source.Exists || source.Attributes.HasFlag(FileAttributes.Directory) ||
            source.Attributes.HasFlag(FileAttributes.ReparsePoint))
        {
            throw new ValidationException("The selected item must be a regular file.");
        }

        if (source.Length <= 0 || source.Length > MaximumBytes)
        {
            throw new ValidationException("Wallpapers must be between 1 byte and 20 MB.");
        }

        var (mimeType, extension) = await DetectTypeAsync(source.FullName, cancellationToken);
        await imageValidator.ValidateAsync(source.FullName, mimeType, cancellationToken);
        await paths.EnsureDirectoriesAsync(cancellationToken);
        var id = RandomNumberGenerator.GetHexString(32).ToLowerInvariant();
        var filename = id + extension;
        var destination = Path.Combine(paths.UserWallpapersDirectory, filename);
        var temporary = Path.Combine(paths.UserWallpapersDirectory, $".{Guid.NewGuid():N}.importing");

        try
        {
            await CopyBoundedAsync(source.FullName, temporary, cancellationToken);
            File.Move(temporary, destination);
            return await writeCoordinator.ExecuteAsync(
                async token =>
                {
                    await using var connection = await connectionFactory.OpenConnectionAsync(token);
                    await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                    try
                    {
                        await using (var insert = connection.CreateCommand())
                        {
                            insert.Transaction = transaction;
                            insert.CommandText = """
                                INSERT INTO user_wallpapers (id, display_name, filename, mime_type, byte_size)
                                VALUES ($id, $name, $filename, $mimeType, $byteSize);
                                """;
                            insert.Parameters.AddWithValue("$id", id);
                            insert.Parameters.AddWithValue("$name", DisplayName(source.Name));
                            insert.Parameters.AddWithValue("$filename", filename);
                            insert.Parameters.AddWithValue("$mimeType", mimeType);
                            insert.Parameters.AddWithValue("$byteSize", source.Length);
                            await insert.ExecuteNonQueryAsync(token);
                        }

                        await UpsertSelectionAsync(connection, transaction, $"custom:{id}", token);
                        await transaction.CommitAsync(token);
                    }
                    catch
                    {
                        try { await transaction.RollbackAsync(CancellationToken.None); } catch { }
                        throw;
                    }

                    var users = await ListUsersAsync(token);
                    return new WallpaperSettings($"custom:{id}", true, [.. BuiltIns, .. users]);
                },
                cancellationToken);
        }
        catch
        {
            TryDelete(temporary);
            TryDelete(destination);
            throw;
        }
    }

    public Task<WallpaperSettings> DeleteAsync(
        string id,
        CancellationToken cancellationToken = default)
    {
        if (!WallpaperIdRegex().IsMatch(id))
        {
            throw new ValidationException("Choose a valid user wallpaper.");
        }

        return writeCoordinator.ExecuteAsync(
            async token =>
            {
                await using var connection = await connectionFactory.OpenConnectionAsync(token);
                string filename;
                await using (var read = connection.CreateCommand())
                {
                    read.CommandText = "SELECT filename FROM user_wallpapers WHERE id = $id;";
                    read.Parameters.AddWithValue("$id", id);
                    filename = await read.ExecuteScalarAsync(token) as string
                        ?? throw new NotFoundException("The user wallpaper was not found.");
                }

                if (!WallpaperFilenameRegex().IsMatch(filename))
                {
                    throw new InvalidOperationException("Stored wallpaper metadata is invalid.");
                }

                var original = Path.Combine(paths.UserWallpapersDirectory, filename);
                var staged = Path.Combine(paths.UserWallpapersDirectory, $".{Guid.NewGuid():N}.deleting");
                var stagedFile = false;
                if (File.Exists(original))
                {
                    File.Move(original, staged);
                    stagedFile = true;
                }

                await using var transaction = (SqliteTransaction)await connection.BeginTransactionAsync(token);
                try
                {
                    await using (var delete = connection.CreateCommand())
                    {
                        delete.Transaction = transaction;
                        delete.CommandText = "DELETE FROM user_wallpapers WHERE id = $id;";
                        delete.Parameters.AddWithValue("$id", id);
                        if (await delete.ExecuteNonQueryAsync(token) != 1)
                        {
                            throw new NotFoundException("The user wallpaper was not found.");
                        }
                    }

                    await using (var selected = connection.CreateCommand())
                    {
                        selected.Transaction = transaction;
                        selected.CommandText = "SELECT value FROM app_state WHERE key = $key;";
                        selected.Parameters.AddWithValue("$key", SelectionStateKey);
                        if (string.Equals(
                            await selected.ExecuteScalarAsync(token) as string,
                            $"custom:{id}",
                            StringComparison.Ordinal))
                        {
                            await UpsertSelectionAsync(connection, transaction, DefaultSelection, token);
                        }
                    }

                    await transaction.CommitAsync(token);
                }
                catch
                {
                    try { await transaction.RollbackAsync(CancellationToken.None); } catch { }
                    if (stagedFile && File.Exists(staged)) File.Move(staged, original);
                    throw;
                }

                if (stagedFile) TryDelete(staged);
                return await GetAsync(token);
            },
            cancellationToken);
    }

    private async Task<IReadOnlyList<WallpaperItem>> ListUsersAsync(CancellationToken cancellationToken)
    {
        await paths.EnsureDirectoriesAsync(cancellationToken);
        await using var connection = await connectionFactory.OpenConnectionAsync(cancellationToken);
        await using var command = connection.CreateCommand();
        command.CommandText = """
            SELECT id, display_name, filename, mime_type, byte_size
            FROM user_wallpapers ORDER BY created_at DESC, id ASC;
            """;
        var result = new List<WallpaperItem>();
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        while (await reader.ReadAsync(cancellationToken))
        {
            var id = reader.GetString(0);
            var filename = reader.GetString(2);
            if (!WallpaperIdRegex().IsMatch(id) || !WallpaperFilenameRegex().IsMatch(filename)) continue;
            var fullPath = Path.Combine(paths.UserWallpapersDirectory, filename);
            var file = new FileInfo(fullPath);
            if (!file.Exists || file.Attributes.HasFlag(FileAttributes.ReparsePoint)) continue;
            result.Add(new WallpaperItem(
                $"custom:{id}", id, reader.GetString(1), "Uploaded image", false,
                fullPath, reader.GetString(3), reader.GetInt64(4)));
        }

        return result;
    }

    private static bool SelectionExists(string selection, IReadOnlyList<WallpaperItem> users) =>
        BuiltIns.Any(item => item.Key == selection) || users.Any(item => item.Key == selection);

    private static async Task<(string MimeType, string Extension)> DetectTypeAsync(
        string path,
        CancellationToken cancellationToken)
    {
        var header = new byte[12];
        await using var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 4096, true);
        if (await stream.ReadAsync(header, cancellationToken) < header.Length)
        {
            throw new ValidationException("Choose a JPEG, PNG, or WebP image.");
        }

        if (header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF)
            return ("image/jpeg", ".jpg");
        ReadOnlySpan<byte> pngSignature = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
        if (header.AsSpan(0, 8).SequenceEqual(pngSignature))
            return ("image/png", ".png");
        if (header.AsSpan(0, 4).SequenceEqual("RIFF"u8) && header.AsSpan(8, 4).SequenceEqual("WEBP"u8))
            return ("image/webp", ".webp");
        throw new ValidationException("Choose a JPEG, PNG, or WebP image.");
    }

    private static async Task CopyBoundedAsync(
        string source,
        string destination,
        CancellationToken cancellationToken)
    {
        await using var input = new FileStream(source, FileMode.Open, FileAccess.Read, FileShare.Read, 81920, true);
        await using var output = new FileStream(destination, FileMode.CreateNew, FileAccess.Write, FileShare.None, 81920, true);
        var buffer = new byte[81920];
        long written = 0;
        int read;
        while ((read = await input.ReadAsync(buffer, cancellationToken)) > 0)
        {
            written += read;
            if (written > MaximumBytes) throw new ValidationException("Wallpapers cannot exceed 20 MB.");
            await output.WriteAsync(buffer.AsMemory(0, read), cancellationToken);
        }

        await output.FlushAsync(cancellationToken);
    }

    private static Task UpsertSelectionAsync(
        SqliteConnection connection,
        SqliteTransaction? transaction,
        string selection,
        CancellationToken cancellationToken)
    {
        var command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = """
            INSERT INTO app_state (key, value) VALUES ($key, $value)
            ON CONFLICT(key) DO UPDATE SET value = excluded.value;
            """;
        command.Parameters.AddWithValue("$key", SelectionStateKey);
        command.Parameters.AddWithValue("$value", selection);
        return ExecuteAndDisposeAsync(command, cancellationToken);
    }

    private static async Task ExecuteAndDisposeAsync(SqliteCommand command, CancellationToken cancellationToken)
    {
        await using (command) await command.ExecuteNonQueryAsync(cancellationToken);
    }

    private static string DisplayName(string filename)
    {
        var name = Path.GetFileNameWithoutExtension(filename).Trim();
        if (name.Length == 0) return "Uploaded wallpaper";
        return string.Concat(name.EnumerateRunes().Take(120));
    }

    private static void TryDelete(string path)
    {
        try { if (File.Exists(path)) File.Delete(path); } catch { }
    }

    [GeneratedRegex("^[a-f0-9]{32}$", RegexOptions.CultureInvariant)]
    private static partial Regex WallpaperIdRegex();

    [GeneratedRegex("^[a-f0-9]{32}\\.(jpg|png|webp)$", RegexOptions.CultureInvariant)]
    private static partial Regex WallpaperFilenameRegex();
}
