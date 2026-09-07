package mcpserver

import (
	"context"
	"log/slog"
	"reflect"
	"sort"
	"testing"

	"something/mcp-server/tools"
	readtools "something/mcp-server/tools/read"
	writetools "something/mcp-server/tools/write"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCompleteMCPToolContractIsRegistered(t *testing.T) {
	protocolServer := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	runner := tools.NewRunner(slog.Default(), "http://127.0.0.1:8080")
	readtools.Register(protocolServer, runner)
	writetools.Register(protocolServer, runner)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := protocolServer.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(result.Tools))
	seen := make(map[string]struct{}, len(result.Tools))
	for _, tool := range result.Tools {
		if _, duplicate := seen[tool.Name]; duplicate {
			t.Fatalf("duplicate MCP tool name %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if tool.Title == "" || tool.Description == "" || tool.InputSchema == nil || tool.Annotations == nil {
			t.Fatalf("incomplete MCP tool contract for %q: %#v", tool.Name, tool)
		}
		got = append(got, tool.Name)
	}

	want := []string{
		"get_health",
		"list_news", "get_news_state", "get_news_windows", "refresh_news",
		"list_telegram_posts", "list_youtube_posts", "list_favorite_telegram_posts", "list_favorite_youtube_posts",
		"refresh_telegram_posts", "refresh_youtube_posts",
		"list_telegram_favorites", "list_categorized_telegram_favorites",
		"list_youtube_favorites", "list_categorized_youtube_favorites",
		"add_telegram_favorite", "remove_telegram_favorite", "assign_telegram_favorite_category",
		"add_youtube_favorite", "remove_youtube_favorite", "assign_youtube_favorite_category",
		"list_favorite_categories", "create_favorite_category", "rename_favorite_category",
		"list_tasks", "list_today_tasks", "list_this_week_tasks",
		"create_task", "update_task", "toggle_task", "toggle_task_subtask", "delete_task",
		"list_notes", "get_note", "create_note", "update_note", "set_note_pinned", "set_note_archived", "delete_note",
		"list_bookmarks", "get_bookmark", "list_bookmark_tags",
		"create_bookmark", "update_bookmark", "set_bookmark_read", "delete_bookmark",
		"get_weather", "get_stored_weather", "save_weather_location", "refresh_stored_weather",
		"list_currency_rates", "get_currency_rate", "list_currency_favorites", "list_currency_favorite_rates",
		"add_currency_favorite", "remove_currency_favorite",
		"list_desktop_apps", "rename_desktop_app", "launch_desktop_app",
		"list_setups", "create_setup", "update_setup", "start_setup", "delete_setup",
		"list_steam_games", "get_steam_game_settings", "list_steam_countries",
		"add_steam_game", "delete_steam_game", "set_steam_game_country", "refresh_steam_games",
		"get_wallpaper_settings", "select_wallpaper", "delete_user_wallpaper",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered MCP tools =\n%v\nwant\n%v", got, want)
	}
}
