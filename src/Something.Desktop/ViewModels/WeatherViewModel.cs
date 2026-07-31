using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.Logging;
using Something.Application.Features.Weather;
using Something.Desktop.Navigation;
using Something.Domain.Exceptions;
using Something.Domain.Models.Weather;

namespace Something.Desktop.ViewModels;

public sealed partial class WeatherViewModel(
    IWeatherService weatherService,
    ILogger<WeatherViewModel> logger) : PageViewModel
{
    private CancellationTokenSource? _pageCancellation;

    public override PageKey PageKey => PageKey.Weather;
    public override string Title => "Weather";
    public override string Subtitle => "Use coordinates for your saved forecast or search any supported city.";

    public bool HasSavedForecast => SavedForecast is not null;
    public bool HasCityForecast => CityForecast is not null;

    [ObservableProperty]
    private string _latitudeText = string.Empty;

    [ObservableProperty]
    private string _longitudeText = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasSavedForecast))]
    private WeatherForecast? _savedForecast;

    [ObservableProperty]
    private string _savedLocationLabel = "No saved location";

    [ObservableProperty]
    private string _savedUpdatedLabel = "Enter coordinates to save a local forecast.";

    [ObservableProperty]
    private string _citySearchText = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasCityForecast))]
    private WeatherForecast? _cityForecast;

    [ObservableProperty]
    private string _cityLocationLabel = string.Empty;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    public override async Task OnNavigatedToAsync(CancellationToken cancellationToken)
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        await LoadStoredThenRefreshAsync(_pageCancellation.Token);
    }

    public override void OnNavigatedFrom()
    {
        _pageCancellation?.Cancel();
        _pageCancellation?.Dispose();
        _pageCancellation = null;
    }

    [RelayCommand]
    private Task LoadCoordinatesAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var latitude = ParseCoordinate(LatitudeText, "latitude");
                var longitude = ParseCoordinate(LongitudeText, "longitude");
                SavedForecast = await weatherService.GetForCoordinatesAsync(latitude, longitude, token);
                SavedLocationLabel = SavedForecast.Timezone.Length > 0
                    ? $"Saved location ({SavedForecast.Timezone})"
                    : "Saved location";
                SavedUpdatedLabel = $"Updated {DateTimeOffset.Now:g}";
                StatusMessage = string.Empty;
            },
            cancellationToken,
            "Weather for these coordinates could not be loaded.");
    }

    [RelayCommand]
    private Task RefreshSavedAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var result = await weatherService.RefreshStoredAsync(token);
                if (!result.Found || result.Forecast is null)
                {
                    throw new ValidationException("Save a coordinate forecast before refreshing it.");
                }

                ApplyStored(result);
                StatusMessage = string.Empty;
            },
            cancellationToken,
            "Saved weather could not be refreshed.");
    }

    [RelayCommand]
    private Task SearchCityAsync(CancellationToken cancellationToken)
    {
        return RunBusyAsync(
            async token =>
            {
                var result = await weatherService.GetForCityAsync(CitySearchText, token);
                if (!result.Found || result.Location is null || result.Forecast is null)
                {
                    CityForecast = null;
                    CityLocationLabel = "No supported city matched that search.";
                    return;
                }

                CityForecast = result.Forecast;
                CityLocationLabel = result.Location.DisplayName;
            },
            cancellationToken,
            "Weather for this city could not be loaded.");
    }

    private async Task LoadStoredThenRefreshAsync(CancellationToken cancellationToken)
    {
        IsBusy = true;
        ErrorMessage = null;
        StatusMessage = string.Empty;
        try
        {
            var stored = await weatherService.GetStoredAsync(cancellationToken);
            if (!stored.Found)
            {
                SavedForecast = null;
                SavedLocationLabel = "No saved location";
                SavedUpdatedLabel = "Enter coordinates to save a local forecast.";
                return;
            }

            if (stored.Forecast is not null)
            {
                ApplyStored(stored);
            }

            try
            {
                var refreshed = await weatherService.RefreshStoredAsync(cancellationToken);
                if (refreshed.Forecast is not null)
                {
                    ApplyStored(refreshed);
                }
            }
            catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
            {
                throw;
            }
            catch (Exception exception) when (stored.Forecast is not null)
            {
                logger.LogWarning(exception, "Weather refresh failed; keeping cached forecast.");
                StatusMessage = "Showing the last saved forecast because refresh is unavailable.";
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception exception)
        {
            HandleError(exception, "Saved weather could not be loaded.");
        }
        finally
        {
            IsBusy = false;
        }
    }

    private void ApplyStored(StoredWeatherResult result)
    {
        SavedForecast = result.Forecast;
        SavedLocationLabel = result.Forecast?.Timezone is { Length: > 0 } timezone
            ? $"Saved location ({timezone})"
            : "Saved location";
        SavedUpdatedLabel = result.UpdatedAt is null
            ? "Cached forecast"
            : $"Updated {result.UpdatedAt.Value.LocalDateTime:g}";
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
            HandleError(exception, fallbackMessage);
        }
        finally
        {
            IsBusy = false;
        }
    }

    private void HandleError(Exception exception, string fallbackMessage)
    {
        logger.LogError(exception, "Weather operation failed.");
        ErrorMessage = exception is ValidationException or ProviderUnavailableException or NotFoundException
            ? exception.Message
            : fallbackMessage;
    }

    private static double ParseCoordinate(string value, string name)
    {
        if (double.TryParse(value, NumberStyles.Float, CultureInfo.CurrentCulture, out var coordinate) ||
            double.TryParse(value, NumberStyles.Float, CultureInfo.InvariantCulture, out coordinate))
        {
            return coordinate;
        }

        throw new ValidationException($"Enter a valid {name}.");
    }
}
