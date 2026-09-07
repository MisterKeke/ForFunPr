package apiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type endpointContract struct {
	name       string
	method     string
	path       string
	query      string
	body       any
	response   string
	statusCode int
	call       func(context.Context, *Client) error
}

func TestEndpointMethodsPreservePublicHTTPContracts(t *testing.T) {
	before := 17
	pinned := true
	tests := []endpointContract{
		{name: "health", method: http.MethodGet, path: "/api/v1/health", response: `{}`, call: func(ctx context.Context, client *Client) error { _, err := client.Health(ctx); return err }},
		{name: "news refresh", method: http.MethodPost, path: "/api/v1/news/refresh", body: map[string]any{}, response: `{}`, call: func(ctx context.Context, client *Client) error { _, err := client.RefreshNews(ctx); return err }},
		{name: "Telegram posts", method: http.MethodGet, path: "/api/v1/posts/telegram/channel", query: "before=17", response: `[]`, call: func(ctx context.Context, client *Client) error {
			_, err := client.TelegramPosts(ctx, "channel", &before)
			return err
		}},
		{name: "YouTube refresh", method: http.MethodPost, path: "/api/v1/posts/youtube/channel/refresh", body: map[string]any{}, response: `[]`, call: func(ctx context.Context, client *Client) error {
			_, err := client.RefreshYouTubePosts(ctx, "channel")
			return err
		}},
		{name: "task filters", method: http.MethodGet, path: "/api/v1/tasks", query: "date=2028-02-29&difficulty=hard&priority=high&q=ship&tag=one&tag=two", response: `[]`, call: func(ctx context.Context, client *Client) error {
			_, err := client.ListTasks(ctx, TaskListFilter{Query: "ship", Date: "2028-02-29", Priority: "high", Difficulty: "hard", Tags: []string{"one", "two"}})
			return err
		}},
		{name: "note filters", method: http.MethodGet, path: "/api/v1/notes", query: "archive=active&limit=20&offset=40&pinned=true&q=release", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.ListNotes(ctx, NoteListFilter{Query: "release", Archive: "active", Pinned: &pinned, Limit: 20, Offset: 40})
			return err
		}},
		{name: "bookmark create", method: http.MethodPost, path: "/api/v1/bookmarks", body: BookmarkWriteRequest{URL: "https://example.com", Title: "Example", Tags: []string{"one"}}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.CreateBookmark(ctx, BookmarkWriteRequest{URL: "https://example.com", Title: "Example", Tags: []string{"one"}})
			return err
		}},
		{name: "categorized favorites", method: http.MethodGet, path: "/api/v1/favorites/telegram/categories", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.Favorites(ctx, "telegram", true)
			return err
		}},
		{name: "favorite assignment", method: http.MethodPut, path: "/api/v1/favorites/youtube/channel/category", body: map[string]any{"category_id": float64(4)}, statusCode: http.StatusNoContent, call: func(ctx context.Context, client *Client) error {
			return client.AssignFavoriteCategory(ctx, "youtube", "channel", 4)
		}},
		{name: "favorite categories", method: http.MethodGet, path: "/api/v1/favorite-categories", query: "source=youtube", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.FavoriteCategories(ctx, "youtube")
			return err
		}},
		{name: "weather city", method: http.MethodGet, path: "/api/v1/weather", query: "city=New+York", response: `{}`, call: func(ctx context.Context, client *Client) error { _, err := client.Weather(ctx, "New York"); return err }},
		{name: "weather save", method: http.MethodPut, path: "/api/v1/weather", body: map[string]any{"latitude": 41.0, "longitude": 29.0}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.SaveWeatherLocation(ctx, WeatherLocationRequest{Latitude: 41, Longitude: 29})
			return err
		}},
		{name: "currencies", method: http.MethodGet, path: "/api/v1/currencies", query: "base=USD&symbols=EUR%2CTRY", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.Currencies(ctx, "USD", "EUR,TRY")
			return err
		}},
		{name: "currency favorite", method: http.MethodPut, path: "/api/v1/currencies/favorites/USD/EUR", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.AddCurrencyFavorite(ctx, "USD", "EUR")
			return err
		}},
		{name: "desktop rename", method: http.MethodPut, path: "/api/v1/desktop-apps/3/name", body: map[string]any{"display_name": "Editor"}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.RenameDesktopApp(ctx, 3, "Editor")
			return err
		}},
		{name: "desktop launch", method: http.MethodPost, path: "/api/v1/desktop-apps/3/launch", body: map[string]any{"confirm": true}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.LaunchDesktopApp(ctx, 3, true)
			return err
		}},
		{name: "setup start", method: http.MethodPost, path: "/api/v1/setups/2/start", body: map[string]any{"confirm": true}, response: `{}`, call: func(ctx context.Context, client *Client) error { _, err := client.StartSetup(ctx, 2, true); return err }},
		{name: "Steam add", method: http.MethodPost, path: "/api/v1/steam-games", body: map[string]any{"store_url": "https://store.steampowered.com/app/10"}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.AddSteamGame(ctx, "https://store.steampowered.com/app/10")
			return err
		}},
		{name: "Steam country", method: http.MethodPut, path: "/api/v1/steam-games/settings", body: map[string]any{"country_code": "TR"}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.SetSteamGameCountry(ctx, "TR")
			return err
		}},
		{name: "wallpaper selection", method: http.MethodPut, path: "/api/v1/wallpapers/selection", body: map[string]any{"selection": "hu-tao"}, response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.SelectWallpaper(ctx, "hu-tao")
			return err
		}},
		{name: "wallpaper delete", method: http.MethodDelete, path: "/api/v1/wallpapers/id", response: `{}`, call: func(ctx context.Context, client *Client) error {
			_, err := client.DeleteUserWallpaper(ctx, "id")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != test.method || request.URL.Path != test.path || request.URL.RawQuery != test.query {
					t.Errorf("request = %s %s", request.Method, request.URL.String())
				}
				assertEndpointBody(t, request, test.body)
				status := test.statusCode
				if status == 0 {
					status = http.StatusOK
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if test.response != "" {
					_, _ = io.WriteString(w, test.response)
				}
			}))
			defer server.Close()
			client, err := New(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.call(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertEndpointBody(t *testing.T, request *http.Request, want any) {
	t.Helper()
	if want == nil {
		if request.Body != nil {
			data, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
				return
			}
			if len(data) != 0 {
				t.Errorf("unexpected request body %q", data)
			}
		}
		return
	}
	var got any
	if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
		t.Errorf("decode request body: %v", err)
		return
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Errorf("encode expected request body: %v", err)
		return
	}
	var normalizedWant any
	if err := json.Unmarshal(wantJSON, &normalizedWant); err != nil {
		t.Errorf("normalize expected request body: %v", err)
		return
	}
	if !reflect.DeepEqual(got, normalizedWant) {
		t.Errorf("request body = %#v, want %#v", got, normalizedWant)
	}
}
