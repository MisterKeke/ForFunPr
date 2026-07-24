package writetools

import (
	"strconv"

	"currency-wails/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register adds every POST-, PUT-, and DELETE-backed Something CLI leaf
// command as an MCP tool. PATCH-backed tools can be added here when the CLI
// gains a corresponding leaf command.
func Register(server *mcp.Server, runner *tools.Runner) {
	RegisterNews(server, runner)
	RegisterFavorites(server, runner)
	RegisterFavoriteCategories(server, runner)
	RegisterTasks(server, runner)
	RegisterWeather(server, runner)
	RegisterCurrencies(server, runner)
}

func positiveInteger(value int) string {
	return strconv.Itoa(value)
}
