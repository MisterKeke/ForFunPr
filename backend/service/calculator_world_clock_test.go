package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestCalculatorExpressionGrammarAndFailures(t *testing.T) {
	service := newFeatureTestService(t)
	cases := []struct {
		expression string
		answer     float64
		want       float64
		approx     bool
	}{
		{"2 + 3 * 4", 0, 14, false},
		{"(2 + 3) * 4", 0, 20, false},
		{"2^3^2", 0, 512, true},
		{"-2^2", 0, -4, true},
		{"200% + 1", 0, 3, false},
		{"sqrt(81) + abs(-2)", 0, 11, true},
		{"ans / 4", 20, 5, false},
		{"pi", 0, math.Pi, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.expression, func(t *testing.T) {
			result, err := service.EvaluateCalculatorExpressionContext(context.Background(), CalculatorExpressionRequest{
				Expression: testCase.expression, PreviousResult: testCase.answer,
			})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(result.Result-testCase.want) > 1e-12 || result.Approximate != testCase.approx || result.HistoryID <= 0 {
				t.Fatalf("result = %#v, want %v approximate=%v", result, testCase.want, testCase.approx)
			}
		})
	}

	for _, expression := range []string{
		"", "1 / 0", "sqrt(-1)", "ln(0)", "1 +", "(1 + 2", "unknown(1)", "1 2",
	} {
		t.Run("invalid "+expression, func(t *testing.T) {
			_, err := service.EvaluateCalculatorExpressionContext(context.Background(), CalculatorExpressionRequest{Expression: expression})
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != "expression" {
				t.Fatalf("error = %v, want expression ValidationError", err)
			}
		})
	}
	if _, err := service.EvaluateCalculatorExpressionContext(context.Background(), CalculatorExpressionRequest{
		Expression: strings.Repeat("1", maximumCalculatorExpressionLength+1),
	}); err == nil {
		t.Fatal("oversized calculator expression accepted")
	}
}

func TestCalculatorConversionsDatesAndHistory(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	celsius, err := service.ConvertCalculatorUnitContext(ctx, UnitConversionRequest{Value: 0, From: "c", To: "f"})
	if err != nil || math.Abs(celsius.Result-32) > 1e-10 {
		t.Fatalf("Celsius conversion = %#v, %v", celsius, err)
	}
	if _, err := service.ConvertCalculatorUnitContext(ctx, UnitConversionRequest{Value: 1, From: "m", To: "kg"}); err == nil {
		t.Fatal("cross-dimension conversion accepted")
	}
	if _, err := service.ConvertCalculatorUnitContext(ctx, UnitConversionRequest{Value: math.Inf(1), From: "m", To: "km"}); err == nil {
		t.Fatal("infinite conversion value accepted")
	}

	difference, err := service.CalculateDateContext(ctx, DateCalculationRequest{
		Operation: "difference", StartDate: "2024-02-28", EndDate: "2024-03-01",
	})
	if err != nil || difference.Days != 2 {
		t.Fatalf("leap-day difference = %#v, %v", difference, err)
	}
	added, err := service.CalculateDateContext(ctx, DateCalculationRequest{
		Operation: "add", StartDate: "2024-02-28", Amount: 1, Unit: "days",
	})
	if err != nil || added.Date != "2024-02-29" {
		t.Fatalf("date addition = %#v, %v", added, err)
	}
	if _, err := service.CalculateDateContext(ctx, DateCalculationRequest{
		Operation: "add", StartDate: "2024-01-01", Amount: 1, Unit: "fortnights",
	}); err == nil {
		t.Fatal("unsupported date unit accepted")
	}

	history, err := service.ListCalculatorHistoryContext(ctx, 10)
	if err != nil || len(history) != 3 {
		t.Fatalf("calculator history = %#v, %v", history, err)
	}
	if err := service.DeleteCalculatorHistoryItemContext(ctx, history[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearCalculatorHistoryContext(ctx); err != nil {
		t.Fatal(err)
	}
	history, err = service.ListCalculatorHistoryContext(ctx, 10)
	if err != nil || len(history) != 0 {
		t.Fatalf("cleared calculator history = %#v, %v", history, err)
	}
}

func TestWorldTimeConversionHandlesDSTAndFractionalOffsets(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	for _, local := range []string{"2024-03-10T02:30", "2024-11-03T01:30"} {
		_, err := service.ConvertWorldTimeContext(ctx, WorldTimeConversionRequest{
			At: local, FromZoneID: "America/New_York", ToZoneIDs: []string{"UTC"},
		})
		var validation *ValidationError
		if !errors.As(err, &validation) || validation.Field != "at" {
			t.Fatalf("DST time %q error = %v", local, err)
		}
	}

	results, err := service.ConvertWorldTimeContext(ctx, WorldTimeConversionRequest{
		At: "2024-11-03T01:30:00-04:00", FromZoneID: "America/New_York",
		ToZoneIDs: []string{"UTC", "Asia/Kathmandu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].LocalTime != "2024-11-03T05:30:00Z" || results[1].UTCOffsetSeconds != 5*3600+45*60 {
		t.Fatalf("world time results = %#v", results)
	}
}

func TestWorldClockCRUDValidationAndAtomicReorder(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	first, err := service.CreateWorldClockContext(ctx, WorldClockWriteRequest{
		Label: "London", TimeZoneID: "Europe/London", WorkingDayStartMinutes: 9 * 60, WorkingDayEndMinutes: 17 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateWorldClockContext(ctx, WorldClockWriteRequest{
		Label: "Tokyo", TimeZoneID: "Asia/Tokyo", WorkingDayStartMinutes: 8 * 60, WorkingDayEndMinutes: 18 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReorderWorldClocksContext(ctx, WorldClockOrderRequest{IDs: []int{second.ID, first.ID}}); err != nil {
		t.Fatal(err)
	}
	clocks, err := service.ListWorldClocksContext(ctx)
	if err != nil || len(clocks) != 2 || clocks[0].ID != second.ID {
		t.Fatalf("reordered clocks = %#v, %v", clocks, err)
	}
	if err := service.ReorderWorldClocksContext(ctx, WorldClockOrderRequest{IDs: []int{first.ID, first.ID}}); err == nil {
		t.Fatal("duplicate reorder IDs accepted")
	}
	clocks, err = service.ListWorldClocksContext(ctx)
	if err != nil || clocks[0].ID != second.ID {
		t.Fatalf("invalid reorder changed order: %#v, %v", clocks, err)
	}
	if _, err := normalizeWorldClockRequest(WorldClockWriteRequest{
		Label: "Invalid", TimeZoneID: "UTC", WorkingDayStartMinutes: 1000, WorkingDayEndMinutes: 900,
	}); err == nil {
		t.Fatal("reversed working hours accepted")
	}
}
