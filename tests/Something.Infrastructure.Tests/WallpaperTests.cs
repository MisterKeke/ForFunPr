using CommunityToolkit.Mvvm.Messaging;
using Microsoft.Extensions.Logging.Abstractions;
using Something.Application.Abstractions.FileSystem;
using Something.Application.Features.Wallpapers;
using Something.Domain.Exceptions;
using Something.Infrastructure.Database;
using Something.Infrastructure.Database.Migrations;
using Something.Infrastructure.Database.Repositories;

namespace Something.Infrastructure.Tests;

public sealed class WallpaperTests
{
    [Fact]
    public async Task ImportSelectAndDeleteUseApplicationOwnedRandomFile()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await WallpaperHarness.CreateAsync(cancellationToken);
        var source = Path.Combine(harness.SourceDirectory, "My picture.not-really-png");
        await File.WriteAllBytesAsync(source,
            [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3, 4],
            cancellationToken);

        var imported = await harness.Service.ImportAsync(source, cancellationToken);
        Assert.StartsWith("custom:", imported.Selected, StringComparison.Ordinal);
        var item = Assert.Single(imported.Wallpapers, wallpaper => !wallpaper.IsBuiltIn);
        Assert.Matches("^[a-f0-9]{32}\\.png$", Path.GetFileName(item.FilePath));
        Assert.True(File.Exists(item.FilePath));
        Assert.Equal("image/png", Assert.Single(harness.Validator.ValidatedMimeTypes));
        Assert.Equal("My picture", item.Label);

        var selected = await harness.Service.SelectAsync("builtin:skirk", cancellationToken);
        Assert.Equal("builtin:skirk", selected.Selected);
        var deleted = await harness.Service.DeleteAsync(item.Id, cancellationToken);
        Assert.DoesNotContain(deleted.Wallpapers, wallpaper => wallpaper.Id == item.Id);
        Assert.False(File.Exists(item.FilePath));
    }

    [Fact]
    public async Task SignatureAndMaximumSizeAreEnforcedBeforeDecoderRuns()
    {
        var cancellationToken = TestContext.Current.CancellationToken;
        await using var harness = await WallpaperHarness.CreateAsync(cancellationToken);
        var spoofed = Path.Combine(harness.SourceDirectory, "spoofed.png");
        await File.WriteAllTextAsync(spoofed, "this is not an image", cancellationToken);
        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.ImportAsync(spoofed, cancellationToken));

        var oversized = Path.Combine(harness.SourceDirectory, "large.png");
        await using (var stream = new FileStream(oversized, FileMode.CreateNew, FileAccess.Write))
        {
            stream.SetLength(20L * 1024 * 1024 + 1);
        }
        await Assert.ThrowsAsync<ValidationException>(() => harness.Service.ImportAsync(oversized, cancellationToken));
        Assert.Empty(harness.Validator.ValidatedMimeTypes);
    }

    private sealed class WallpaperHarness : IAsyncDisposable
    {
        private readonly TemporaryApplicationPaths _paths;
        private readonly DatabaseInitializer _initializer;
        private readonly DatabaseWriteCoordinator _coordinator;

        private WallpaperHarness(
            TemporaryApplicationPaths paths,
            DatabaseInitializer initializer,
            DatabaseWriteCoordinator coordinator,
            RecordingWallpaperValidator validator,
            WallpaperService service)
        {
            _paths = paths;
            _initializer = initializer;
            _coordinator = coordinator;
            Validator = validator;
            Service = service;
            SourceDirectory = Path.Combine(paths.DataDirectory, "sources");
            Directory.CreateDirectory(SourceDirectory);
        }

        public string SourceDirectory { get; }
        public RecordingWallpaperValidator Validator { get; }
        public WallpaperService Service { get; }

        public static async Task<WallpaperHarness> CreateAsync(CancellationToken cancellationToken)
        {
            var paths = TemporaryApplicationPaths.Create();
            var connectionFactory = new SqliteConnectionFactory(paths);
            var migrations = new IMigration[]
            {
                new Migration001CoreSchema(), new Migration002FavoriteSchema(),
                new Migration003ApplicationState(), new Migration004FavoriteUpdates(),
                new Migration005WeatherLocation(), new Migration006WeatherCache(),
                new Migration007FavoriteNews(), new Migration008TaskMetadata(),
                new Migration009Wallpapers(), new Migration010Notes(), new Migration011Bookmarks(),
            };
            var initializer = new DatabaseInitializer(
                paths, connectionFactory, migrations, NullLogger<DatabaseInitializer>.Instance);
            await initializer.InitializeAsync(cancellationToken);
            var coordinator = new DatabaseWriteCoordinator();
            var validator = new RecordingWallpaperValidator();
            var repository = new WallpaperRepository(paths, connectionFactory, coordinator, validator);
            var service = new WallpaperService(repository, new WeakReferenceMessenger());
            return new WallpaperHarness(paths, initializer, coordinator, validator, service);
        }

        public async ValueTask DisposeAsync()
        {
            _initializer.Dispose();
            _coordinator.Dispose();
            await _paths.DisposeAsync();
        }
    }

    private sealed class RecordingWallpaperValidator : IWallpaperImageValidator
    {
        public List<string> ValidatedMimeTypes { get; } = [];

        public Task ValidateAsync(
            string filePath,
            string mimeType,
            CancellationToken cancellationToken = default)
        {
            cancellationToken.ThrowIfCancellationRequested();
            Assert.True(File.Exists(filePath));
            ValidatedMimeTypes.Add(mimeType);
            return Task.CompletedTask;
        }
    }
}
