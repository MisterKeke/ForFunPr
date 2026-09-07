package api

import (
	"net/http"
	"testing"
)

func TestLocalReadRoutesReturnStableSuccessEnvelopes(t *testing.T) {
	router := newRouter(newReadyAPITestService(t))
	paths := []string{
		"/api/v1/news",
		"/api/v1/news/state",
		"/api/v1/news/windows",
		"/api/v1/posts/favorites/telegram",
		"/api/v1/posts/favorites/youtube",
		"/api/v1/favorites/telegram",
		"/api/v1/favorites/telegram/categories",
		"/api/v1/favorites/youtube",
		"/api/v1/favorites/youtube/categories",
		"/api/v1/favorite-categories",
		"/api/v1/tasks/today",
		"/api/v1/tasks/week",
		"/api/v1/notes",
		"/api/v1/bookmarks",
		"/api/v1/bookmarks/tags",
		"/api/v1/weather/stored",
		"/api/v1/currencies/favorites",
		"/api/v1/currencies/favorites/rates",
		"/api/v1/desktop-apps",
		"/api/v1/setups",
		"/api/v1/steam-games",
		"/api/v1/steam-games/settings",
		"/api/v1/steam-games/countries",
		"/api/v1/wallpapers",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := performAPIRequest(router, http.MethodGet, path, "")
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
			}
			if response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
				response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("GET %s headers = %#v", path, response.Header())
			}
		})
	}
}

func TestProviderAndExecutionRoutesRejectInvalidInputBeforeExternalWork(t *testing.T) {
	router := newRouter(newReadyAPITestService(t))
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{name: "Telegram pagination", method: http.MethodGet, path: "/api/v1/posts/telegram/channel?before=word", status: http.StatusBadRequest, code: "invalid_before"},
		{name: "YouTube pagination", method: http.MethodGet, path: "/api/v1/posts/youtube/@channel?before=1", status: http.StatusUnprocessableEntity, code: "youtube_pagination_unsupported"},
		{name: "Telegram refresh channel", method: http.MethodPost, path: "/api/v1/posts/telegram/bad!/refresh", body: `{}`, status: http.StatusBadRequest, code: "invalid_telegram_channel"},
		{name: "YouTube refresh channel", method: http.MethodPost, path: "/api/v1/posts/youtube/bad!/refresh", body: `{}`, status: http.StatusBadRequest, code: "invalid_youtube_channel"},
		{name: "Telegram favorite", method: http.MethodPut, path: "/api/v1/favorites/telegram/bad!", body: `{}`, status: http.StatusBadRequest, code: "invalid_telegram_channel"},
		{name: "YouTube favorite", method: http.MethodPut, path: "/api/v1/favorites/youtube/bad!", body: `{}`, status: http.StatusBadRequest, code: "invalid_youtube_channel"},
		{name: "favorite assignment", method: http.MethodPut, path: "/api/v1/favorites/telegram/channel/category", body: `{"category_id":0}`, status: http.StatusUnprocessableEntity, code: "invalid_category_id"},
		{name: "favorite category", method: http.MethodPost, path: "/api/v1/favorite-categories", body: `{"name":"","source":"telegram"}`, status: http.StatusUnprocessableEntity, code: "invalid_category_name"},
		{name: "weather city", method: http.MethodGet, path: "/api/v1/weather", status: http.StatusBadRequest, code: "invalid_city"},
		{name: "weather coordinates", method: http.MethodPut, path: "/api/v1/weather", body: `{"latitude":91,"longitude":0}`, status: http.StatusUnprocessableEntity, code: "invalid_coordinates"},
		{name: "missing stored weather", method: http.MethodPost, path: "/api/v1/weather/stored/refresh", body: `{}`, status: http.StatusNotFound, code: "stored_location_not_found"},
		{name: "currency list", method: http.MethodGet, path: "/api/v1/currencies", status: http.StatusBadRequest, code: "invalid_base_currency"},
		{name: "currency rate", method: http.MethodGet, path: "/api/v1/currencies/rate?base=USD&target=nope", status: http.StatusBadRequest, code: "invalid_currency_pair"},
		{name: "identical currency favorite", method: http.MethodPut, path: "/api/v1/currencies/favorites/USD/USD", body: `{}`, status: http.StatusUnprocessableEntity, code: "identical_currency_pair"},
		{name: "desktop app ID", method: http.MethodPut, path: "/api/v1/desktop-apps/nope/name", body: `{"display_name":"Name"}`, status: http.StatusBadRequest, code: "invalid_desktop_app_id"},
		{name: "desktop launch confirmation", method: http.MethodPost, path: "/api/v1/desktop-apps/1/launch", body: `{"confirm":false}`, status: http.StatusUnprocessableEntity, code: "execution_confirmation_required"},
		{name: "setup", method: http.MethodPost, path: "/api/v1/setups", body: `{"name":"","app_ids":[]}`, status: http.StatusUnprocessableEntity, code: "invalid_setup"},
		{name: "setup launch confirmation", method: http.MethodPost, path: "/api/v1/setups/1/start", body: `{"confirm":false}`, status: http.StatusUnprocessableEntity, code: "execution_confirmation_required"},
		{name: "Steam URL", method: http.MethodPost, path: "/api/v1/steam-games", body: `{"store_url":"https://example.com/game"}`, status: http.StatusUnprocessableEntity, code: "invalid_steam_game"},
		{name: "Steam country", method: http.MethodPut, path: "/api/v1/steam-games/settings", body: `{"country_code":"XX"}`, status: http.StatusUnprocessableEntity, code: "invalid_steam_game"},
		{name: "Steam ID", method: http.MethodDelete, path: "/api/v1/steam-games/nope", status: http.StatusBadRequest, code: "invalid_steam_game_id"},
		{name: "wallpaper selection", method: http.MethodPut, path: "/api/v1/wallpapers/selection", body: `{"selection":"missing"}`, status: http.StatusUnprocessableEntity, code: "invalid_wallpaper"},
		{name: "wallpaper ID", method: http.MethodDelete, path: "/api/v1/wallpapers/not-a-valid-id", status: http.StatusUnprocessableEntity, code: "invalid_wallpaper"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performAPIRequest(router, test.method, test.path, test.body)
			if response.Code != test.status || errorCode(t, response) != test.code {
				t.Fatalf("%s %s = %d %s", test.method, test.path, response.Code, response.Body.String())
			}
		})
	}
}

func TestNewsAndEmptySteamRefreshMutationContracts(t *testing.T) {
	router := newRouter(newReadyAPITestService(t))
	for _, path := range []string{
		"/api/v1/news/initial",
		"/api/v1/news/refresh",
		"/api/v1/news/since-last-open",
		"/api/v1/steam-games/refresh",
	} {
		t.Run(path, func(t *testing.T) {
			response := performAPIRequest(router, http.MethodPost, path, `{}`)
			if response.Code != http.StatusOK {
				t.Fatalf("POST %s = %d %s", path, response.Code, response.Body.String())
			}
		})
	}
}
