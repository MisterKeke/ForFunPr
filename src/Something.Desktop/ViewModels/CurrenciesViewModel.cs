using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Currencies;
using Something.Desktop.Navigation;
using Something.Domain.Exceptions;
using Something.Domain.Models.Currencies;

namespace Something.Desktop.ViewModels;

public sealed partial class CurrenciesViewModel(
    ICurrencyService currencyService,
    ILogger<CurrenciesViewModel> logger) : PageViewModel
{
    private CancellationTokenSource? _pageCancellation;
    private IReadOnlyList<CurrencyRateItem> _allRates = [];

    public override PageKey PageKey => PageKey.Currencies;
    public override string Title => "Currencies";
    public override string Subtitle => "Check current exchange rates and keep favorite pairs close by.";

    public ObservableCollection<CurrencyRateItem> FilteredRates { get; } = [];
    public ObservableCollection<FavoriteCurrencyRate> FavoriteRates { get; } = [];

    [ObservableProperty]
    private string _baseCode = "EUR";

    [ObservableProperty]
    private string _targetCode = "USD";

    [ObservableProperty]
    private string _conversionAmount = "1";

    [ObservableProperty]
    private string _singleRateText = string.Empty;

    [ObservableProperty]
    private string _singleRateMeta = string.Empty;

    [ObservableProperty]
    private string _conversionText = string.Empty;

    [ObservableProperty]
    private string _allRatesBaseCode = "EUR";

    [ObservableProperty]
    private string _allRatesSummary = string.Empty;

    [ObservableProperty]
    private string _rateSearchText = string.Empty;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await RunBusyAsync(LoadFavoritesCoreAsync, _pageCancellation.Token, "Favorite rates could not be loaded.");
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }

    [RelayCommand]
    private Task LoadSingleRateAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var rate = await currencyService.GetRateAsync(BaseCode, TargetCode, token);
                SingleRateText = rate.Found
                    ? $"1 {rate.Base} = {rate.Rate:0.####} {rate.Target}"
                    : "No rate found";
                SingleRateMeta = rate.Found
                    ? rate.Date is null ? "Same currency" : $"Rate date: {rate.Date:yyyy-MM-dd}"
                    : $"{rate.Base} to {rate.Target}";
            },
            cancellationToken,
            "Exchange rate could not be loaded.");
    }

    [RelayCommand]
    private void SwapCurrencies()
    {
        (BaseCode, TargetCode) = (TargetCode, BaseCode);
    }

    [RelayCommand]
    private Task ConvertAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                if (!decimal.TryParse(
                        ConversionAmount,
                        NumberStyles.Number,
                        CultureInfo.CurrentCulture,
                        out var amount) || amount <= 0)
                {
                    throw new ValidationException("Enter a positive conversion amount.");
                }

                var rate = await currencyService.GetRateAsync(BaseCode, TargetCode, token);
                ConversionText = rate.Found
                    ? $"{amount:0.####} {rate.Base} = {amount * rate.Rate:0.####} {rate.Target}"
                    : "No rate found";
            },
            cancellationToken,
            "Conversion rate could not be loaded.");
    }

    [RelayCommand]
    private Task LoadAllRatesAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var result = await currencyService.GetAllRatesAsync(AllRatesBaseCode, token);
                _allRates = result.Rates;
                AllRatesSummary = $"{result.Rates.Count} rates for {result.Base}" +
                    (result.Date is null ? string.Empty : $" on {result.Date:yyyy-MM-dd}");
                RateSearchText = string.Empty;
                ApplyRateFilter();
            },
            cancellationToken,
            "Exchange rates could not be loaded.");
    }

    [RelayCommand]
    private Task AddFavoriteAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await currencyService.AddFavoriteAsync($"{BaseCode}:{TargetCode}", token);
                await LoadFavoritesCoreAsync(token);
            },
            cancellationToken,
            "Favorite pair could not be saved.");
    }

    [RelayCommand]
    private Task RemoveFavoriteAsync(FavoriteCurrencyRate favorite, CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                await currencyService.RemoveFavoriteAsync(favorite.Code, token);
                await LoadFavoritesCoreAsync(token);
            },
            cancellationToken,
            "Favorite pair could not be removed.");
    }

    [RelayCommand]
    private Task RefreshFavoritesAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(LoadFavoritesCoreAsync, cancellationToken, "Favorite rates could not be refreshed.");
    }

    partial void OnRateSearchTextChanged(string value) => ApplyRateFilter();

    private async Task LoadFavoritesCoreAsync(CancellationToken cancellationToken)
    {
        var favorites = await currencyService.GetFavoritesWithRatesAsync(cancellationToken);
        FavoriteRates.Clear();
        foreach (var favorite in favorites)
        {
            FavoriteRates.Add(favorite);
        }
    }

    private void ApplyRateFilter()
    {
        var query = RateSearchText.Trim().ToUpperInvariant();
        FilteredRates.Clear();
        foreach (var rate in _allRates.Where(rate =>
                     query.Length == 0 || rate.Code.Contains(query, StringComparison.Ordinal)))
        {
            FilteredRates.Add(rate);
        }
    }

    private async Task RunBusyAsync(
        Func<CancellationToken, Task> operation,
        CancellationToken cancellationToken,
        string fallbackMessage)
    {
        using var linked = _pageCancellation is null
            ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken)
            : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, _pageCancellation.Token);
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
            logger.LogError(exception, "Currency operation failed.");
            ErrorMessage = exception is ValidationException or ProviderUnavailableException
                ? exception.Message
                : fallbackMessage;
        }
        finally
        {
            IsBusy = false;
        }
    }
}
