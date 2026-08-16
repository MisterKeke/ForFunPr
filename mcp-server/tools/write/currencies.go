package writetools

import (
	"context"

	"something/mcp-server/schemas"
	"something/mcp-server/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterCurrencies adds local currency favorite mutations.
func RegisterCurrencies(server *mcp.Server, runner *tools.Runner) {
	tools.AddTool(server, &mcp.Tool{
		Name:        "add_currency_favorite",
		Title:       "Add currency favorite",
		Description: "Add a local currency pair favorite or confirm it already exists.",
		InputSchema: schemas.CurrencyPairInputSchema,
		Annotations: tools.WriteAnnotations(false, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CurrencyPairInput,
	) (*mcp.CallToolResult, schemas.AddCurrencyFavoriteOutput, error) {
		base, target, err := currencyPair(input)
		if err != nil {
			return nil, schemas.AddCurrencyFavoriteOutput{}, err
		}
		if base == target {
			return nil, schemas.AddCurrencyFavoriteOutput{},
				&currencyInputError{
					message: "base and target currencies must be different",
				}
		}
		return tools.Execute[schemas.AddCurrencyFavoriteOutput](
			ctx,
			runner,
			[]string{
				"currencies", "favorites", "add",
				"--base", base,
				"--target", target,
			},
			"Added the currency favorite, or confirmed it already existed.",
		)
	})

	tools.AddTool(server, &mcp.Tool{
		Name:        "remove_currency_favorite",
		Title:       "Remove currency favorite",
		Description: "Remove a local saved currency pair.",
		InputSchema: schemas.CurrencyPairInputSchema,
		Annotations: tools.WriteAnnotations(true, true, false),
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		input schemas.CurrencyPairInput,
	) (*mcp.CallToolResult, schemas.StatusOutput, error) {
		base, target, err := currencyPair(input)
		if err != nil {
			return nil, schemas.StatusOutput{}, err
		}
		return tools.Execute[schemas.StatusOutput](
			ctx,
			runner,
			[]string{
				"currencies", "favorites", "remove",
				"--base", base,
				"--target", target,
			},
			"Removed the requested currency favorite.",
		)
	})
}

func currencyPair(
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

type currencyInputError struct {
	message string
}

func (err *currencyInputError) Error() string {
	return err.message
}
