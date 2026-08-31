package readtools

import (
	"context"
	"fmt"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSteamGames(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{Name: "list_steam_games", Title: "List Steam games", Description: "List locally tracked Steam games and cached prices.", InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false)}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.SteamGamesOutput, error) {
		items, err := tools.Run[[]schemas.SteamGame](ctx, runner, []string{"steam-games", "list"})
		return tools.Response(fmt.Sprintf("Listed %d tracked Steam games.", len(items)), schemas.SteamGamesOutput{Games: items}, err)
	})
	tools.AddTool(server, &mcp.Tool{Name: "get_steam_game_settings", Title: "Get Steam settings", Description: "Read the country used for Steam price requests.", InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false)}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.SteamGameSettings, error) {
		return tools.Execute[schemas.SteamGameSettings](ctx, runner, []string{"steam-games", "settings"}, "Loaded Steam price settings.")
	})
	tools.AddTool(server, &mcp.Tool{Name: "list_steam_countries", Title: "List Steam countries", Description: "List supported Steam Store price countries.", InputSchema: schemas.EmptyInputSchema, Annotations: tools.ReadAnnotations(false)}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.SteamCountriesOutput, error) {
		items, err := tools.Run[[]schemas.SteamCountry](ctx, runner, []string{"steam-games", "countries"})
		return tools.Response(fmt.Sprintf("Listed %d supported Steam countries.", len(items)), schemas.SteamCountriesOutput{Countries: items}, err)
	})
}
