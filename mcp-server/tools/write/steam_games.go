package writetools

import (
	"context"
	"strings"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSteamGames(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{Name: "add_steam_game", Title: "Add Steam game", Description: "Fetch and track one public Steam Store game URL.", InputSchema: schemas.AddSteamGameInputSchema, Annotations: tools.WriteAnnotations(false, false, true)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.AddSteamGameInput) (*mcp.CallToolResult, schemas.SteamGame, error) {
		storeURL, err := schemas.RequiredString("store_url", input.StoreURL)
		if err != nil {
			return nil, schemas.SteamGame{}, err
		}
		return tools.Execute[schemas.SteamGame](ctx, runner, []string{"steam-games", "add", "--url", storeURL}, "Added the Steam game to local tracking.")
	})
	tools.AddTool(server, &mcp.Tool{Name: "delete_steam_game", Title: "Delete Steam game", Description: "Permanently remove a tracked Steam game and its cached artwork.", InputSchema: schemas.SteamGameIDInputSchema, Annotations: tools.WriteAnnotations(true, true, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.SteamGameIDInput) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		id, err := schemas.PositiveID("id", input.ID)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](ctx, runner, []string{"steam-games", "delete", "--id", positiveInteger(id)}, "Deleted the tracked Steam game.")
	})
	tools.AddTool(server, &mcp.Tool{Name: "set_steam_game_country", Title: "Set Steam country", Description: "Overwrite the country used by future Steam price requests.", InputSchema: schemas.SteamCountryInputSchema, Annotations: tools.WriteAnnotations(true, true, false)}, func(ctx context.Context, _ *mcp.CallToolRequest, input schemas.SteamCountryInput) (*mcp.CallToolResult, schemas.SteamGameSettings, error) {
		country, err := schemas.RequiredString("country_code", input.CountryCode)
		if err != nil {
			return nil, schemas.SteamGameSettings{}, err
		}
		return tools.Execute[schemas.SteamGameSettings](ctx, runner, []string{"steam-games", "set-country", "--country", strings.ToUpper(country)}, "Saved the Steam price country.")
	})
	tools.AddTool(server, &mcp.Tool{Name: "refresh_steam_games", Title: "Refresh Steam games", Description: "Fetch current Steam data for every tracked game and update the local price cache.", InputSchema: schemas.EmptyInputSchema, Annotations: tools.WriteAnnotations(false, false, true)}, func(ctx context.Context, _ *mcp.CallToolRequest, _ schemas.EmptyInput) (*mcp.CallToolResult, schemas.SteamGameRefreshResult, error) {
		return tools.Execute[schemas.SteamGameRefreshResult](ctx, runner, []string{"steam-games", "refresh"}, "Refreshed tracked Steam game data.")
	})
}
