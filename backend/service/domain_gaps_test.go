package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCurrencyFavoriteCRUDAndLiveResponseContracts(t *testing.T) {
	service := newFeatureTestService(t)
	added, err := service.AddFavorite(" usd : eur ")
	if err != nil || added.Pair != "USD:EUR" || !added.Added || added.Exists {
		t.Fatalf("first add = %#v, %v", added, err)
	}
	duplicate, err := service.AddFavorite("USD:EUR")
	if err != nil || duplicate.Added || !duplicate.Exists {
		t.Fatalf("duplicate add = %#v, %v", duplicate, err)
	}
	if _, err := service.AddFavorite("USD:USD"); err == nil {
		t.Fatal("identical favorite currencies were accepted")
	}
	favorites, err := service.ListFavorites()
	if err != nil || !reflect.DeepEqual(favorites, []string{"USD:EUR"}) {
		t.Fatalf("favorites = %#v, %v", favorites, err)
	}
	if removed, err := service.RemoveFavorite("usd:eur"); err != nil || removed != "USD:EUR" {
		t.Fatalf("remove = %q, %v", removed, err)
	}

	requests := 0
	service.httpClient.client.Transport = testRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		var payload string
		switch request.URL.Path {
		case "/v2/rate/USD/EUR":
			payload = `{"date":"2026-09-07","base":"USD","quote":"EUR","rate":0.85}`
		case "/v2/rates":
			if request.URL.Query().Get("base") != "USD" {
				t.Errorf("rates base query = %q", request.URL.Query().Get("base"))
			}
			payload = `[{"date":"2026-09-07","base":"USD","quote":"TRY","rate":42},{"date":"2026-09-07","base":"USD","quote":"EUR","rate":0.85}]`
		default:
			t.Fatalf("unexpected currency request %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(payload)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})

	identity, err := service.GetRateContext(context.Background(), "try", "TRY")
	if err != nil || identity.Rate != 1 || !identity.Found || requests != 0 {
		t.Fatalf("identity rate = %#v, %v, requests=%d", identity, err, requests)
	}
	rate, err := service.GetRateContext(context.Background(), "usd", "eur")
	if err != nil || rate.Base != "USD" || rate.To != "EUR" || rate.Rate != 0.85 || !rate.Found {
		t.Fatalf("rate = %#v, %v", rate, err)
	}
	all, err := service.GetAllRatesContext(context.Background(), "usd")
	if err != nil || !reflect.DeepEqual(all.Codes, []string{"EUR", "TRY"}) || all.Rates["TRY"] != 42 {
		t.Fatalf("all rates = %#v, %v", all, err)
	}
}

func TestWeatherCacheRoundTripAndCorruptionFallback(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	missing, err := service.GetStoredLocationWeather()
	if err != nil || missing.Found {
		t.Fatalf("initial stored weather = %#v, %v", missing, err)
	}
	weather := &WeatherResult{
		Current: WeatherCurrent{Temperature2M: 21, WeatherCode: 1},
		Daily: WeatherDaily{
			Time: []string{"2026-09-07", "2026-09-08"}, WeatherCode: []int{1, 2},
			Temperature2MMax: []float64{24, 25}, Temperature2MMin: []float64{16, 17},
			PrecipitationProbabilityMax: []float64{10, 20},
		},
		Timezone: "Europe/Istanbul",
	}
	if err := service.saveWeatherLocationAndForecastContext(ctx, 41.0082, 28.9784, weather); err != nil {
		t.Fatal(err)
	}
	stored, err := service.GetStoredLocationWeather()
	if err != nil || !stored.Found || stored.Weather == nil || stored.Weather.Timezone != "Europe/Istanbul" || stored.UpdatedAt == "" {
		t.Fatalf("stored weather = %#v, %v", stored, err)
	}

	if _, err := service.db.ExecContext(ctx, `UPDATE location SET forecast_json = ? WHERE id = 1`, `{"daily":{}}`); err != nil {
		t.Fatal(err)
	}
	stored, err = service.GetStoredLocationWeather()
	if err != nil || !stored.Found || stored.Weather != nil {
		t.Fatalf("corrupt cache fallback = %#v, %v", stored, err)
	}

	fresh := newFeatureTestService(t)
	refreshed, err := fresh.RefreshStoredLocationWeatherContext(ctx)
	if err != nil || refreshed.Found {
		t.Fatalf("refresh without location = %#v, %v", refreshed, err)
	}
}

func TestWeatherProviderFlowAndFailedRefreshPreserveCache(t *testing.T) {
	service := newFeatureTestService(t)
	forecastPayload := `{
		"current":{"temperature_2m":21,"apparent_temperature":20,"weather_code":1,"wind_speed_10m":5},
		"daily":{"time":["2026-09-07"],"weather_code":[1],"temperature_2m_max":[24],"temperature_2m_min":[16],"precipitation_probability_max":[10]},
		"timezone":"Europe/Istanbul"
	}`
	service.httpClient.client.Transport = testRoundTripper(func(request *http.Request) (*http.Response, error) {
		payload := forecastPayload
		if request.URL.Hostname() == "geocoding-api.open-meteo.com" {
			if request.URL.Query().Get("name") != "Istanbul" {
				t.Errorf("geocoding query = %q", request.URL.Query().Get("name"))
			}
			payload = `{"results":[{"id":1,"name":"Istanbul","latitude":41.0082,"longitude":28.9784,"admin1":"Istanbul","country":"Türkiye"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(payload)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})

	city, err := service.GetWeatherForCityContext(context.Background(), " Istanbul ")
	if err != nil || !city.Found || city.Location.Name != "Istanbul" || city.Weather == nil || city.Weather.Current.Temperature2M != 21 {
		t.Fatalf("city weather = %#v, %v", city, err)
	}
	if _, err := service.GetWeatherContext(context.Background(), 41.0082, 28.9784); err != nil {
		t.Fatal(err)
	}

	service.httpClient.client.Transport = testRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("provider offline")
	})
	if _, err := service.RefreshStoredLocationWeatherContext(context.Background()); err == nil {
		t.Fatal("failed provider refresh returned success")
	}
	stored, err := service.GetStoredLocationWeather()
	if err != nil || stored.Weather == nil || stored.Weather.Current.Temperature2M != 21 {
		t.Fatalf("cache after failed refresh = %#v, %v", stored, err)
	}
}

func TestDesktopAppCRUDValidatesPathsAndAvailability(t *testing.T) {
	useTemporaryApplicationData(t)
	service := newFeatureTestService(t)
	ctx := context.Background()
	firstPath := filepath.Join(t.TempDir(), "First App.exe")
	secondPath := filepath.Join(t.TempDir(), "Second App.exe")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("executable"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	created, err := service.AddDesktopAppFromPathContext(ctx, firstPath)
	if err != nil || created.DisplayName != "First App" || !created.Available {
		t.Fatalf("created app = %#v, %v", created, err)
	}
	if _, err := service.AddDesktopAppFromPathContext(ctx, firstPath); err == nil {
		t.Fatal("duplicate executable path was accepted")
	}
	iconPath := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(iconPath, testPNG(t, 4, 4), 0o600); err != nil {
		t.Fatal(err)
	}
	withIcon, err := service.ImportDesktopAppIconFromPathContext(ctx, created.ID, iconPath)
	if err != nil || withIcon.IconURL == "" {
		t.Fatalf("app with icon = %#v, %v", withIcon, err)
	}
	withoutIcon, err := service.DeleteDesktopAppIconContext(ctx, created.ID)
	if err != nil || withoutIcon.IconURL != "" {
		t.Fatalf("app after icon deletion = %#v, %v", withoutIcon, err)
	}
	renamed, err := service.RenameDesktopAppContext(ctx, created.ID, " Editor ")
	if err != nil || renamed.DisplayName != "Editor" {
		t.Fatalf("renamed app = %#v, %v", renamed, err)
	}
	relocated, err := service.RelocateDesktopAppContext(ctx, created.ID, secondPath)
	if err != nil || relocated.ExecutablePath != secondPath || !relocated.Available {
		t.Fatalf("relocated app = %#v, %v", relocated, err)
	}
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	apps, err := service.ListDesktopAppsContext(ctx)
	if err != nil || len(apps) != 1 || apps[0].Available {
		t.Fatalf("apps after executable removal = %#v, %v", apps, err)
	}
	if _, err := service.DesktopAppExecutableContext(ctx, created.ID); err == nil {
		t.Fatal("missing executable was returned for launch")
	}
	if err := service.DeleteDesktopAppContext(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetDesktopAppContext(ctx, created.ID); err == nil {
		t.Fatal("deleted desktop app was still readable")
	}
}

func TestSteamSettingsLocalListRefreshAndDelete(t *testing.T) {
	useTemporaryApplicationData(t)
	service := newFeatureTestService(t)
	ctx := context.Background()
	settings, err := service.GetSteamGameSettingsContext(ctx)
	if err != nil || settings.CountryCode != "TR" {
		t.Fatalf("default Steam settings = %#v, %v", settings, err)
	}
	settings, err = service.SetSteamGameCountryContext(ctx, " us ")
	if err != nil || settings.CountryCode != "US" {
		t.Fatalf("updated Steam settings = %#v, %v", settings, err)
	}
	if _, err := service.SetSteamGameCountryContext(ctx, "XX"); err == nil {
		t.Fatal("unsupported Steam country was accepted")
	}

	result, err := service.db.ExecContext(ctx, `
		INSERT INTO steam_games (
			steam_app_id, store_url, name, image_source_url, price_status,
			price_country_code, last_refresh_error
		) VALUES (?, ?, ?, '', 'unavailable', ?, '')
	`, 10, "https://store.steampowered.com/app/10", "Counter-Strike", "US")
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	games, err := service.ListSteamGamesContext(ctx)
	if err != nil || len(games) != 1 || games[0].SteamAppID != 10 || games[0].Name != "Counter-Strike" {
		t.Fatalf("Steam games = %#v, %v", games, err)
	}
	service.httpClient.client.Transport = testRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("Steam unavailable")
	})
	failedRefresh, err := service.RefreshSteamGamesContext(ctx)
	if err != nil || failedRefresh.Attempted != 1 || failedRefresh.Failed != 1 || failedRefresh.Updated != 0 ||
		len(failedRefresh.Errors) != 1 || len(failedRefresh.Games) != 1 || failedRefresh.Games[0].LastRefreshError == "" {
		t.Fatalf("failed Steam refresh = %#v, %v", failedRefresh, err)
	}
	if err := service.DeleteSteamGameContext(ctx, int(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSteamGameContext(ctx, service.db, int(id)); err == nil {
		t.Fatal("deleted Steam game was still readable")
	}
	refresh, err := service.RefreshSteamGamesContext(ctx)
	if err != nil || refresh.Attempted != 0 || refresh.Updated != 0 || refresh.Failed != 0 || len(refresh.Games) != 0 {
		t.Fatalf("empty Steam refresh = %#v, %v", refresh, err)
	}
}

func TestAppStateWindowMapping(t *testing.T) {
	state := appState{
		PreviousOpenedAt: "previous-open", CurrentOpenedAt: "current-open",
		PreviousRefreshAt: "previous-refresh", LastRefreshAt: "last-refresh",
	}
	result := favoriteUpdateStateFromAppState(state)
	if result.PreviousOpenedAt != state.PreviousOpenedAt || result.LastRefreshAt != state.LastRefreshAt ||
		result.UpdateWindows.NewWhileClosed.PublishedAfter != "previous-open" ||
		result.UpdateWindows.NewWhileClosed.PublishedUntil != "current-open" ||
		result.UpdateWindows.NewWhileOpen.PublishedAfter != "previous-refresh" ||
		result.UpdateWindows.NewWhileOpen.PublishedUntil != "last-refresh" {
		t.Fatalf("favorite update state = %#v", result)
	}
}

func TestRecordAppOpenRollsForwardPersistentWindows(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	for key, value := range map[string]string{
		"previous_opened_at":  "2026-09-01T08:00:00Z",
		"current_opened_at":   "2026-09-06T08:00:00Z",
		"previous_refresh_at": "2026-09-06T09:00:00Z",
		"last_refresh_at":     "2026-09-06T10:00:00Z",
	} {
		if err := upsertAppStateValue(ctx, service.db, key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.RecordAppOpen(); err != nil {
		t.Fatal(err)
	}
	state, err := service.GetFavoriteUpdateState()
	if err != nil {
		t.Fatal(err)
	}
	if state.PreviousOpenedAt != "2026-09-06T08:00:00Z" || state.CurrentOpenedAt == "" ||
		state.CurrentOpenedAt == state.PreviousOpenedAt ||
		state.PreviousRefreshAt != "2026-09-06T09:00:00Z" || state.LastRefreshAt != "2026-09-06T10:00:00Z" {
		t.Fatalf("rolled app state = %#v", state)
	}
}

func TestDesktopAndSteamNotFoundErrorsRemainTyped(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	for _, err := range []error{
		service.DeleteDesktopAppContext(ctx, 999999),
		service.DeleteSteamGameContext(ctx, 999999),
	} {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("error = %v, want NotFoundError", err)
		}
	}
}
