package backend

import (
	"fmt"
	"strings"
)

func normalizeCollectionPage(
	limit int,
	offset int,
	defaultLimit int,
	maximumLimit int,
) (int, int, error) {
	if offset < 0 {
		return 0, 0, &ValidationError{
			Field: "offset", Message: "offset must be zero or greater",
		}
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maximumLimit {
		return 0, 0, &ValidationError{
			Field: "limit",
			Message: fmt.Sprintf("limit must be between 1 and %d", maximumLimit),
		}
	}
	return limit, offset, nil
}

func collectionLikePattern(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	).Replace(value)
	return "%" + value + "%"
}

func boolDatabaseValue(value bool) int {
	if value {
		return 1
	}
	return 0
}
