using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Favorites;
using Something.Application.Features.Posts;
using Something.Desktop.Navigation;
using Something.Domain.Enums;
using Something.Domain.Exceptions;
using Something.Domain.Models.Favorites;
using Something.Domain.Models.Posts;

namespace Something.Desktop.ViewModels;

public sealed partial class TelegramViewModel(
    ITelegramService telegramService,
    IFavoriteCategoryService categoryService,
    ILogger<TelegramViewModel> logger) : PageViewModel
{
    private const int MaximumDisplayedPosts = 100;
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.Telegram;
    public override string Title => "Telegram";
    public override string Subtitle => "Read public channel posts and organize favorite channels locally.";

    public ObservableCollection<TelegramPost> Posts { get; } = [];
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
        await RunBusyAsync(LoadFavoritesAndCategoriesAsync, _pageCancellation!.Token, "Telegram favorites could not be loaded.");
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
        IsCategoryManagerOpen = false;
    }

    [RelayCommand]
    private Task LoadPostsAsync(CancellationToken cancellationToken)
    {
        return LoadPostsCoreAsync(forceRefresh: false, cancellationToken);
    }

    [RelayCommand]
    private Task RefreshPostsAsync(CancellationToken cancellationToken)
    {
        return LoadPostsCoreAsync(forceRefresh: true, cancellationToken);
    }

    [RelayCommand]
    private Task LoadOlderPostsAsync(CancellationToken cancellationToken)
    {
        var before = Posts
            .Select(static post => post.PostId.Split('/').LastOrDefault())
            .Select(static value => int.TryParse(value, out var id) ? id : 0)
            .Where(static id => id > 0)
            .DefaultIfEmpty(0)
            .Min();
        if (before == 0)
        {
            ErrorMessage = "The current posts do not include a pagination cursor.";
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                var older = await telegramService.GetOlderPostsAsync(ChannelInput, before, token);
                ReplacePosts(Posts.Concat(older));
            },
            cancellationToken,
            "Older Telegram posts could not be loaded.");
    }

    [RelayCommand]
    private Task AddFavoriteAsync(CancellationToken cancellationToken)
    {
        if (SelectedCategory is null)
        {
            ErrorMessage = "Choose or create a Telegram category first.";
            return Task.CompletedTask;
        }

        return RunBusyAsync(
            async token =>
            {
                await telegramService.AddFavoriteAsync(ChannelInput, SelectedCategory.Id, token);
                await LoadFavoritesAndCategoriesAsync(token);
            },
            cancellationToken,
            "Telegram favorite could not be saved.");
    }

    [RelayCommand]
    private Task RemoveFavoriteAsync(FavoriteChannelViewModel favorite, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await telegramService.RemoveFavoriteAsync(favorite.SourceId, token);
                await LoadFavoritesAndCategoriesAsync(token);
            },
            cancellationToken,
            "Telegram favorite could not be removed.");
    }

    [RelayCommand]
    private async Task SelectFavoriteAsync(FavoriteChannelViewModel favorite, CancellationToken cancellationToken)
    {
        ChannelInput = favorite.SourceId;
        await LoadPostsCoreAsync(forceRefresh: false, cancellationToken);
    }

    [RelayCommand]
    private Task CreateCategoryAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var category = await categoryService.CreateAsync(
                    NewCategoryName, FavoriteSource.Telegram, token);
                NewCategoryName = string.Empty;
                await LoadFavoritesAndCategoriesAsync(token);
                SelectedCategory = Categories.FirstOrDefault(item => item.Id == category.Id);
            },
            cancellationToken,
            "Telegram category could not be created.");
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
            "Telegram category could not be renamed.");
    }

    private Task LoadPostsCoreAsync(bool forceRefresh, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token => ReplacePosts(await telegramService.GetPostsAsync(ChannelInput, forceRefresh, token)),
            cancellationToken,
            "Telegram posts could not be loaded.");
    }

    private async Task LoadFavoritesAndCategoriesAsync(CancellationToken cancellationToken)
    {
        var categories = await categoryService.ListAsync(FavoriteSource.Telegram, cancellationToken);
        var favorites = await telegramService.ListFavoritesAsync(cancellationToken);
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

    private void ReplacePosts(IEnumerable<TelegramPost> posts)
    {
        var normalized = posts
            .DistinctBy(static post => post.PostId.Length > 0 ? post.PostId : $"{post.PublishedAt:O}|{post.Text}")
            .OrderByDescending(static post => post.PublishedAt ?? DateTimeOffset.MinValue)
            .Take(MaximumDisplayedPosts)
            .ToArray();
        Posts.Clear();
        foreach (var post in normalized)
        {
            Posts.Add(post);
        }

        IsEmpty = Posts.Count == 0;
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
            logger.LogError(exception, "Telegram operation failed.");
            ErrorMessage = exception is ValidationException or ProviderUnavailableException or NotFoundException or ConflictException
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
