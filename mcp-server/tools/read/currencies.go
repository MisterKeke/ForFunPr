package readtools

import (
	"context"
	"strings"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterCurrencies adds live rate and local favorite read tools.
func RegisterCurrencies(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "list_currency_rates",
		Title:       "List currency rates",
		Description: "Retrieve live rates for a base currency and optional target symbols.",
		InputSchema: schemas.CurrencyListInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CurrencyListInput,
	) (*mcp.CallToolResult, schemas.CurrencyRatesOutput, error) {
		base, err := schemas.CurrencyCode("base", input.Base)
		if err != nil {
			return nil, schemas.CurrencyRatesOutput{}, err
		}
		symbols, err := schemas.CurrencyCodes(input.Symbols)
		if err != nil {
			return nil, schemas.CurrencyRatesOutput{}, err
		}

		args := []string{"currencies", "list", "--base", base}
		if len(symbols) > 0 {
			args = append(args, "--symbols", strings.Join(symbols, ","))
		}
		return tools.Execute[schemas.CurrencyRatesOutput](
			ctx,
			runner,
			args,
			"Loaded live currency rates.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "get_currency_rate",
		Title:       "Get currency rate",
		Description: "Retrieve one live base-to-target currency rate.",
		InputSchema: schemas.CurrencyPairInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CurrencyPairInput,
	) (*mcp.CallToolResult, schemas.CurrencyRateOutput, error) {
		base, target, err := normalizeCurrencyPair(input)
		if err != nil {
			return nil, schemas.CurrencyRateOutput{}, err
		}
		return tools.Execute[schemas.CurrencyRateOutput](
			ctx,
			runner,
			[]string{
				"currencies", "rate",
				"--base", base,
				"--target", target,
			},
			"Loaded the requested live currency rate.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "list_currency_favorites",
		Title:       "List currency favorites",
		Description: "List locally saved currency pairs without loading rates.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.CurrencyFavoritesOutput, error) {
		return tools.Execute[schemas.CurrencyFavoritesOutput](
			ctx,
			runner,
			[]string{"currencies", "favorites", "list"},
			"Listed saved currency pairs.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "list_currency_favorite_rates",
		Title:       "List favorite currency rates",
		Description: "Retrieve live rates for all locally saved currency pairs.",
		InputSchema: schemas.EmptyInputSchema,
		Annotations: tools.ReadAnnotations(true),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ schemas.EmptyInput,
	) (*mcp.CallToolResult, schemas.CurrencyFavoriteRatesOutput, error) {
		return tools.Execute[schemas.CurrencyFavoriteRatesOutput](
			ctx,
			runner,
			[]string{"currencies", "favorites", "rates"},
			"Loaded live rates for saved currency pairs.",
		)
	})
}

func normalizeCurrencyPair(
	input schemas.CurrencyPairInput,
) (string, string, error) {
	base, err := schemas.CurrencyCode("base", input.Base)
	if err != nil {
		return "", "", err
	}
	target, err := schemas.CurrencyCode("target", input.Target)
	if err != nil {
		return "", "", err
	}
	return base, target, nil
}
