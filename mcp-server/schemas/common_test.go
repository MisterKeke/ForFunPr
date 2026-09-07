package schemas

import (
	"math"
	"reflect"
	"testing"
)

func TestCommonSchemaNormalizers(t *testing.T) {
	if got, err := RequiredString("name", " value "); err != nil || got != "value" {
		t.Fatalf("RequiredString = %q, %v", got, err)
	}
	if _, err := RequiredString("name", " \t "); err == nil {
		t.Fatal("RequiredString accepted blank input")
	}
	if got := OptionalString(" value "); got != "value" {
		t.Fatalf("OptionalString = %q", got)
	}
	if got, err := PositiveID("id", 3); err != nil || got != 3 {
		t.Fatalf("PositiveID = %d, %v", got, err)
	}
	if _, err := PositiveID("id", 0); err == nil {
		t.Fatal("PositiveID accepted zero")
	}

	zero := 0
	negative := -1
	if got, err := PaginationCursor(&zero); err != nil || got == nil || *got != 0 {
		t.Fatalf("PaginationCursor(0) = %#v, %v", got, err)
	}
	if _, err := PaginationCursor(&negative); err == nil {
		t.Fatal("PaginationCursor accepted a negative value")
	}
	if got, err := PaginationCursor(nil); err != nil || got != nil {
		t.Fatalf("PaginationCursor(nil) = %#v, %v", got, err)
	}
}

func TestCommonSchemaDomainValidation(t *testing.T) {
	if got, err := OptionalDate("due_date", " 2028-02-29 "); err != nil || got != "2028-02-29" {
		t.Fatalf("OptionalDate = %q, %v", got, err)
	}
	if _, err := OptionalDate("due_date", "2027-02-29"); err == nil {
		t.Fatal("OptionalDate accepted an invalid calendar date")
	}
	if got, err := OptionalPriority(" HIGH "); err != nil || got != "high" {
		t.Fatalf("OptionalPriority = %q, %v", got, err)
	}
	if _, err := OptionalPriority("urgent"); err == nil {
		t.Fatal("OptionalPriority accepted an unsupported value")
	}
	if got, err := OptionalDifficultyFilter(" UNSET "); err != nil || got != "unset" {
		t.Fatalf("OptionalDifficultyFilter = %q, %v", got, err)
	}
	if got, err := FavoriteSource(" YouTube ", true); err != nil || got != "youtube" {
		t.Fatalf("FavoriteSource = %q, %v", got, err)
	}
	if got, err := CurrencyCode("base", " try "); err != nil || got != "TRY" {
		t.Fatalf("CurrencyCode = %q, %v", got, err)
	}
	if _, err := CurrencyCode("base", "EURO"); err == nil {
		t.Fatal("CurrencyCode accepted a non-three-letter code")
	}

	codes, err := CurrencyCodes([]string{" usd ", "EUR", "USD"})
	if err != nil || !reflect.DeepEqual(codes, []string{"USD", "EUR"}) {
		t.Fatalf("CurrencyCodes = %#v, %v", codes, err)
	}
	if _, err := CurrencyCodes([]string{"USD", "12A"}); err == nil {
		t.Fatal("CurrencyCodes accepted an invalid symbol")
	}
}

func TestCoordinatesRejectNonFiniteAndOutOfRangeValues(t *testing.T) {
	for _, coordinates := range [][2]float64{{-90, -180}, {0, 0}, {90, 180}} {
		if err := Coordinates(coordinates[0], coordinates[1]); err != nil {
			t.Fatalf("Coordinates(%v) = %v", coordinates, err)
		}
	}
	for _, coordinates := range [][2]float64{
		{math.NaN(), 0}, {math.Inf(1), 0}, {91, 0},
		{0, math.NaN()}, {0, math.Inf(-1)}, {0, 181},
	} {
		if err := Coordinates(coordinates[0], coordinates[1]); err == nil {
			t.Fatalf("Coordinates(%v) accepted invalid values", coordinates)
		}
	}
}
