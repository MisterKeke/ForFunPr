package schemas

import (
	"fmt"
	"math"
	"strings"

	"currency-wails/backend"
)

// EmptyInput is used by CLI leaf commands that accept no model-controlled
// arguments.
type EmptyInput struct{}

// EmptyInputSchema is the explicit MCP schema for tools with no arguments.
var EmptyInputSchema = map[string]any{
	"type":                 "object",
	"properties":           map[string]any{},
	"additionalProperties": false,
}

// StatusOutput is the JSON success envelope emitted by CLI delete and
// assignment commands.
type StatusOutput struct {
	Status  string `json:"status" jsonschema:"Success status returned by the Something CLI."`
	Message string `json:"message" jsonschema:"Human-readable mutation result."`
}

// RequiredString trims a required string and rejects empty values.
func RequiredString(label string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

// OptionalString trims an optional string.
func OptionalString(value string) string {
	return strings.TrimSpace(value)
}

// PositiveID validates a database identifier.
func PositiveID(label string, value int) (int, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return value, nil
}

// PaginationCursor validates an optional post pagination cursor.
func PaginationCursor(value *int) (*int, error) {
	if value != nil && *value < 0 {
		return nil, fmt.Errorf("before must be a non-negative integer")
	}
	return value, nil
}

// OptionalDate validates an optional task date in YYYY-MM-DD format.
func OptionalDate(label string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	normalized, err := backend.NormalizeDate(value, false)
	if err != nil {
		return "", fmt.Errorf("%s must use YYYY-MM-DD", label)
	}
	return normalized, nil
}

// OptionalPriority validates an optional task priority.
func OptionalPriority(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}

	return backend.NormalizeTodoPriority(value)
}

func OptionalDifficulty(value string) (string, error) {
	return backend.NormalizeTodoDifficulty(value)
}

func OptionalDifficultyFilter(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "unset" {
		return value, nil
	}
	return backend.NormalizeTodoDifficulty(value)
}

func OptionalTags(values []string) ([]string, error) {
	return backend.NormalizeTodoTags(values)
}

// FavoriteSource validates and normalizes a favorite source.
func FavoriteSource(value string, required bool) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" && !required {
		return "", nil
	}

	return backend.NormalizeFavoriteSource(value)
}

// CurrencyCode validates and uppercases a three-letter ASCII currency code.
func CurrencyCode(label string, value string) (string, error) {
	value, err := backend.NormalizeCurrencyCode(value)
	if err != nil {
		return "", fmt.Errorf("%s must be exactly three ASCII letters", label)
	}
	return value, nil
}

// CurrencyCodes validates, uppercases, and de-duplicates currency codes.
func CurrencyCodes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		code, err := CurrencyCode("each symbol", value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	return normalized, nil
}

// Coordinates validates a latitude/longitude pair.
func Coordinates(latitude float64, longitude float64) error {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
		longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}
