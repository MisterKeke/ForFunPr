using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Abstractions.Dialogs;
using Something.Application.Features.Favorites;
using Something.Application.Features.Posts;
using Something.Desktop.Navigation;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Favorites;
using Something.Domain.Models.Posts;

namespace Something.Desktop.ViewModels;

public sealed partial class YouTubeViewModel(
    IYouTubeService youTubeService,
    IFavoriteCategoryService categoryService,
    IExternalUriLauncher uriLauncher,
    ILogger<YouTubeViewModel> logger) : PageViewModel
{
    private const int MaximumDisplayedVideos = 50;
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.YouTube;
    public override string Title => "YouTube";
    public override string Subtitle => "Read public channel RSS feeds and organize favorite channels locally.";

    public ObservableCollection<YouTubeVideo> Videos { get; } = [];
    public ObservableCollection<FavoriteChannelViewModel> Favorites { get; } = [];
    public ObservableCollection<FavoriteCategory> Categories { get; } = [];
    public ObservableCollection<FavoriteCategoryDraftViewModel> CategoryDrafts { get; } = [];

    [ObservableProperty]
    private string _channelInput = string.Empty;

    [ObservableProperty]
    private FavoriteCategory? _selectedCategory;

    [ObservableProperty]
    private string _newCategoryName = string.Empty;

    [ObservableProperty]
    private bool _isCategoryManagerOpen;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        ResetPageCancellation(cancellationToken);
        await RunBusyAsync(LoadFavoritesAndCategoriesAsync, _pageCancellation!.Token, "YouTube favorites could not be loaded.");
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
        IsCategoryManagerOpen = false;
    }

    [RelayCommand]
    private Task LoadVideosAsync(CancellationToken cancellationToken) =>
        LoadVideosCoreAsync(forceRefresh: false, cancellationToken);

    [RelayCommand]
    private Task RefreshVideosAsync(CancellationToken cancellationToken) =>
        LoadVideosCoreAsync(forceRefresh: true, cancellationToken);

    [RelayCommand]
    private Task OpenVideoAsync(YouTubeVideo video, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            token => uriLauncher.OpenAsync(video.VideoUrl, token),
            cancellationToken,
            "Video could not be opened.");
    }

    [RelayCommand]
    private Task AddFavoriteAsync(CancellationToken cancellationToken)
    {
        if (SelectedCategory is null)
        {
            ErrorMessage = "Choose or create a YouTube category first.";
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                await youTubeService.AddFavoriteAsync(ChannelInput, SelectedCategory.Id, token);
                await LoadFavoritesAndCategoriesAsync(token);
            },
            cancellationToken,
            "YouTube favorite could not be saved.");
    }

    [RelayCommand]
    private Task RemoveFavoriteAsync(FavoriteChannelViewModel favorite, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await youTubeService.RemoveFavoriteAsync(favorite.SourceId, token);
                await LoadFavoritesAndCategoriesAsync(token);
            },
            cancellationToken,
            "YouTube favorite could not be removed.");
    }

    [RelayCommand]
    private async Task SelectFavoriteAsync(FavoriteChannelViewModel favorite, CancellationToken cancellationToken)
    {
        ChannelInput = favorite.DisplayName;
        await LoadVideosCoreAsync(forceRefresh: false, cancellationToken);
    }

    [RelayCommand]
    private Task CreateCategoryAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var category = await categoryService.CreateAsync(
                    NewCategoryName, FavoriteSource.YouTube, token);
                NewCategoryName = string.Empty;
                await LoadFavoritesAndCategoriesAsync(token);
                SelectedCategory = Categories.FirstOrDefault(item => item.Id == category.Id);
            },
            cancellationToken,
            "YouTube category could not be created.");
    }

    [RelayCommand]
    private void OpenCategoryManager() => IsCategoryManagerOpen = true;

    [RelayCommand]
    private void CloseCategoryManager() => IsCategoryManagerOpen = false;

    [RelayCommand]
    private Task RenameCategoryAsync(FavoriteCategoryDraftViewModel draft, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await categoryService.RenameAsync(draft.Id, draft.Name, token);
                await LoadFavoritesAndCategoriesAsync(token);
            },
            cancellationToken,
            "YouTube category could not be renamed.");
    }

    private Task LoadVideosCoreAsync(bool forceRefresh, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var videos = await youTubeService.GetVideosAsync(ChannelInput, forceRefresh, token);
                Videos.Clear();
                foreach (var video in videos
                             .DistinctBy(static video => video.VideoId)
                             .OrderByDescending(static video => video.PublishedAt ?? DateTimeOffset.MinValue)
                             .Take(MaximumDisplayedVideos))
                {
                    Videos.Add(video);
                }

                IsEmpty = Videos.Count == 0;
            },
            cancellationToken,
            "YouTube videos could not be loaded.");
    }

    private async Task LoadFavoritesAndCategoriesAsync(CancellationToken cancellationToken)
    {
        var categories = await categoryService.ListAsync(FavoriteSource.YouTube, cancellationToken);
        var favorites = await youTubeService.ListFavoritesAsync(cancellationToken);
        var selectedId = SelectedCategory?.Id;
        Categories.Clear();
        CategoryDrafts.Clear();
        foreach (var category in categories)
        {
            Categories.Add(category);
            CategoryDrafts.Add(new FavoriteCategoryDraftViewModel(category));
        }

        SelectedCategory = Categories.FirstOrDefault(item => item.Id == selectedId) ?? Categories.FirstOrDefault();
        Favorites.Clear();
        foreach (var favorite in favorites)
        {
            var categoryName = categories.FirstOrDefault(item => item.Id == favorite.CategoryId)?.Name ?? string.Empty;
            Favorites.Add(new FavoriteChannelViewModel(favorite, categoryName));
        }
    }

    private async Task RunBusyAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken,
        string fallbackMessage)
    {
        using var linked = CreateLinked(cancellationToken);
        IsBusy = true;
        ErrorMessage = null;
        try
        {
            await operation(linked.Token);
        }
        catch (OperationCanceledException) when (linked.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            logger.LogError(exception, "YouTube operation failed.");
            ErrorMessage = exception is ValidationException or ProviderUnavailableException or NotFoundException or ConflictException or NotSupportedException
                ? exception.Message
                : fallbackMessage;
        }
        finally
        {
            IsBusy = false;
        }
    }

    private CancellationTokenSource CreateLinked(CancellationToken cancellationToken) =>
        _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);

    private void ResetPageCancellation(CancellationToken cancellationToken)
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
    }
}
