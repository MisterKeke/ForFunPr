package schemas

// CurrencyListInput selects a base and optional target symbols.
type CurrencyListInput struct {
	Base    string   `json:"base"`
	Symbols []string `json:"symbols,omitempty"`
}

// CurrencyListInputSchema is the explicit MCP schema for listing currency
// rates.
var CurrencyListInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"base": map[string]any{
			"type":        "string",
			"description": "Three-letter base currency code.",
			"pattern":     `^[A-Za-z]{3}$`,
		},
		"symbols": map[string]any{
			"type":        "array",
			"description": "Optional list of three-letter target currency codes.",
			"items": map[string]any{
				"type":    "string",
				"pattern": `^[A-Za-z]{3}$`,
			},
		},
	},
	"required":             []string{"base"},
	"additionalProperties": false,
}

// CurrencyPairInput selects one base/target currency pair.
type CurrencyPairInput struct {
	Base   string `json:"base"`
	Target string `json:"target"`
}

// CurrencyPairInputSchema is the explicit MCP schema for one currency pair.
var CurrencyPairInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"base": map[string]any{
			"type":        "string",
			"description": "Three-letter base currency code.",
			"pattern":     `^[A-Za-z]{3}$`,
		},
		"target": map[string]any{
			"type":        "string",
			"description": "Three-letter target currency code.",
			"pattern":     `^[A-Za-z]{3}$`,
		},
	},
	"required":             []string{"base", "target"},
	"additionalProperties": false,
}

// CurrencyRatesOutput is the JSON emitted by `currencies list`. Codes is
// absent only when the CLI requested an explicit symbol subset.
type CurrencyRatesOutput struct {
	Base  string             `json:"base" jsonschema:"Normalized base currency code."`
	Date  string             `json:"date" jsonschema:"Rate date."`
	Rates map[string]float64 `json:"rates" jsonschema:"Rates keyed by target currency code."`
	Codes *[]string          `json:"codes,omitempty" jsonschema:"Available target codes when all rates were requested."`
}

// CurrencyRateOutput is the JSON emitted by `currencies rate`.
type CurrencyRateOutput struct {
	Base  string  `json:"base" jsonschema:"Normalized base currency code."`
	Date  string  `json:"date" jsonschema:"Rate date, or empty when not applicable."`
	To    string  `json:"to" jsonschema:"Normalized target currency code."`
	Rate  float64 `json:"rate" jsonschema:"Exchange rate."`
	Found bool    `json:"found" jsonschema:"Whether the provider returned a rate."`
}

// CurrencyFavoritesOutput is the JSON emitted by the favorite list command.
type CurrencyFavoritesOutput struct {
	Favorites []string `json:"favorites" jsonschema:"Saved BASE:TARGET currency pairs."`
}

// CurrencyFavoriteRate is one favorite pair with its latest rate.
type CurrencyFavoriteRate struct {
	Code  string  `json:"code" jsonschema:"Saved BASE:TARGET pair."`
	Base  string  `json:"base" jsonschema:"Base currency code."`
	To    string  `json:"to" jsonschema:"Target currency code."`
	Rate  float64 `json:"rate" jsonschema:"Latest rate when found."`
	Found bool    `json:"found" jsonschema:"Whether a rate was found."`
}

// CurrencyFavoriteRatesOutput is the JSON emitted by favorites rates.
type CurrencyFavoriteRatesOutput struct {
	Base      string                 `json:"base" jsonschema:"Reserved aggregate base value emitted by the backend."`
	Favorites []CurrencyFavoriteRate `json:"favorites" jsonschema:"Saved pairs and their latest rates."`
}

// AddCurrencyFavoriteOutput is the JSON emitted by favorites add.
type AddCurrencyFavoriteOutput struct {
	Pair   string `json:"pair" jsonschema:"Normalized BASE:TARGET pair."`
	Added  bool   `json:"added" jsonschema:"Whether this call inserted the pair."`
	Exists bool   `json:"exists" jsonschema:"Whether the pair already existed."`
	Error  string `json:"error,omitempty" jsonschema:"Optional safe mutation error."`
}
