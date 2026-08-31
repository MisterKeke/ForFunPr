package readtools

import (
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register adds every GET-backed Something CLI leaf command as an MCP tool.
func Register(server *mcp.Server, runner *tools.Runner) {
	RegisterHealth(server, runner)
	RegisterNews(server, runner)
	RegisterPosts(server, runner)
	RegisterFavorites(server, runner)
	RegisterFavoriteCategories(server, runner)
	RegisterTasks(server, runner)
	RegisterNotes(server, runner)
	RegisterBookmarks(server, runner)
	RegisterWeather(server, runner)
	RegisterCurrencies(server, runner)
	RegisterDesktopApps(server, runner)
	RegisterSetups(server, runner)
	RegisterSteamGames(server, runner)
	RegisterWallpapers(server, runner)
}
