package schemas

import (
	"fmt"
	"math"
	"strings"
	"time"
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

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", fmt.Errorf("%s must use YYYY-MM-DD", label)
	}
	return value, nil
}

// OptionalPriority validates an optional task priority.
func OptionalPriority(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}

	switch value {
	case "low", "medium", "high":
		return value, nil
	default:
		return "", fmt.Errorf("priority must be low, medium, or high")
	}
}

// FavoriteSource validates and normalizes a favorite source.
func FavoriteSource(value string, required bool) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" && !required {
		return "", nil
	}

	switch value {
	case "telegram", "youtube":
		return value, nil
	default:
		return "", fmt.Errorf("source must be telegram or youtube")
	}
}

// CurrencyCode validates and uppercases a three-letter ASCII currency code.
func CurrencyCode(label string, value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return "", fmt.Errorf("%s must be exactly three ASCII letters", label)
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return "", fmt.Errorf("%s must be exactly three ASCII letters", label)
		}
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
