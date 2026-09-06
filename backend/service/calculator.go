package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const maximumCalculatorExpressionLength = 512

type CalculatorExpressionRequest struct {
	Expression     string  `json:"expression"`
	PreviousResult float64 `json:"previous_result"`
}

type CalculatorResult struct {
	Result      float64 `json:"result"`
	ResultText  string  `json:"result_text"`
	Approximate bool    `json:"approximate"`
	HistoryID   int64   `json:"history_id"`
}

type UnitConversionRequest struct {
	Value float64 `json:"value"`
	From  string  `json:"from"`
	To    string  `json:"to"`
}

type UnitOption struct {
	ID        string `json:"id"`
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Dimension string `json:"dimension"`
}

type DateCalculationRequest struct {
	Operation string `json:"operation"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Amount    int    `json:"amount"`
	Unit      string `json:"unit"`
}

type DateCalculationResult struct {
	ResultText string `json:"result_text"`
	Date       string `json:"date,omitempty"`
	Days       int    `json:"days,omitempty"`
	HistoryID  int64  `json:"history_id"`
}

type CalculatorHistoryItem struct {
	ID          int    `json:"id"`
	Mode        string `json:"mode"`
	InputText   string `json:"input_text"`
	ResultText  string `json:"result_text"`
	Approximate bool   `json:"approximate"`
	CreatedAt   string `json:"created_at"`
}

type calculatorUnit struct {
	ID        string
	Symbol    string
	Name      string
	Dimension string
	Scale     float64
	Offset    float64
}

var calculatorUnits = map[string]calculatorUnit{
	"mm":     {"mm", "mm", "Millimetres", "length", 0.001, 0},
	"cm":     {"cm", "cm", "Centimetres", "length", 0.01, 0},
	"m":      {"m", "m", "Metres", "length", 1, 0},
	"km":     {"km", "km", "Kilometres", "length", 1000, 0},
	"in":     {"in", "in", "Inches", "length", 0.0254, 0},
	"ft":     {"ft", "ft", "Feet", "length", 0.3048, 0},
	"yd":     {"yd", "yd", "Yards", "length", 0.9144, 0},
	"mi":     {"mi", "mi", "Miles", "length", 1609.344, 0},
	"mg":     {"mg", "mg", "Milligrams", "mass", 0.000001, 0},
	"g":      {"g", "g", "Grams", "mass", 0.001, 0},
	"kg":     {"kg", "kg", "Kilograms", "mass", 1, 0},
	"oz":     {"oz", "oz", "Ounces", "mass", 0.028349523125, 0},
	"lb":     {"lb", "lb", "Pounds", "mass", 0.45359237, 0},
	"c":      {"c", "°C", "Celsius", "temperature", 1, 273.15},
	"f":      {"f", "°F", "Fahrenheit", "temperature", 5.0 / 9.0, 255.3722222222222},
	"k":      {"k", "K", "Kelvin", "temperature", 1, 0},
	"m2":     {"m2", "m²", "Square metres", "area", 1, 0},
	"km2":    {"km2", "km²", "Square kilometres", "area", 1000000, 0},
	"ft2":    {"ft2", "ft²", "Square feet", "area", 0.09290304, 0},
	"acre":   {"acre", "acre", "Acres", "area", 4046.8564224, 0},
	"ml":     {"ml", "mL", "Millilitres", "volume", 0.001, 0},
	"l":      {"l", "L", "Litres", "volume", 1, 0},
	"gal_us": {"gal_us", "US gal", "US gallons", "volume", 3.785411784, 0},
	"mps":    {"mps", "m/s", "Metres per second", "speed", 1, 0},
	"kph":    {"kph", "km/h", "Kilometres per hour", "speed", 1.0 / 3.6, 0},
	"mph":    {"mph", "mph", "Miles per hour", "speed", 0.44704, 0},
	"s":      {"s", "s", "Seconds", "duration", 1, 0},
	"min":    {"min", "min", "Minutes", "duration", 60, 0},
	"h":      {"h", "h", "Hours", "duration", 3600, 0},
	"day":    {"day", "days", "Days", "duration", 86400, 0},
	"b":      {"b", "B", "Bytes", "storage", 1, 0},
	"kb":     {"kb", "KB", "Kilobytes", "storage", 1000, 0},
	"mb":     {"mb", "MB", "Megabytes", "storage", 1000000, 0},
	"gb":     {"gb", "GB", "Gigabytes", "storage", 1000000000, 0},
	"kib":    {"kib", "KiB", "Kibibytes", "storage", 1024, 0},
	"mib":    {"mib", "MiB", "Mebibytes", "storage", 1048576, 0},
	"gib":    {"gib", "GiB", "Gibibytes", "storage", 1073741824, 0},
}

func (a *Service) EvaluateCalculatorExpressionContext(ctx context.Context, request CalculatorExpressionRequest) (CalculatorResult, error) {
	expression := strings.TrimSpace(request.Expression)
	if expression == "" || len([]rune(expression)) > maximumCalculatorExpressionLength {
		return CalculatorResult{}, &ValidationError{Field: "expression", Message: "expression must contain between 1 and 512 characters"}
	}
	parser := calculationParser{input: expression, answer: request.PreviousResult}
	value, err := parser.parse()
	if err != nil {
		return CalculatorResult{}, &ValidationError{Field: "expression", Message: err.Error()}
	}
	resultText := formatCalculatorNumber(value)
	historyID, err := a.recordCalculatorHistory(ctx, "expression", expression, resultText, parser.approximate)
	if err != nil {
		return CalculatorResult{}, err
	}
	return CalculatorResult{Result: value, ResultText: resultText, Approximate: parser.approximate, HistoryID: historyID}, nil
}

func (a *Service) ListCalculatorUnitsContext(context.Context) []UnitOption {
	units := make([]UnitOption, 0, len(calculatorUnits))
	for _, unit := range calculatorUnits {
		units = append(units, UnitOption{ID: unit.ID, Symbol: unit.Symbol, Name: unit.Name, Dimension: unit.Dimension})
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].Dimension == units[j].Dimension {
			return units[i].Name < units[j].Name
		}
		return units[i].Dimension < units[j].Dimension
	})
	return units
}

func (a *Service) ConvertCalculatorUnitContext(ctx context.Context, request UnitConversionRequest) (CalculatorResult, error) {
	if math.IsNaN(request.Value) || math.IsInf(request.Value, 0) {
		return CalculatorResult{}, &ValidationError{Field: "value", Message: "conversion value must be finite"}
	}
	from, found := calculatorUnits[strings.ToLower(strings.TrimSpace(request.From))]
	if !found {
		return CalculatorResult{}, &ValidationError{Field: "from", Message: "source unit is not supported"}
	}
	to, found := calculatorUnits[strings.ToLower(strings.TrimSpace(request.To))]
	if !found {
		return CalculatorResult{}, &ValidationError{Field: "to", Message: "target unit is not supported"}
	}
	if from.Dimension != to.Dimension {
		return CalculatorResult{}, &ValidationError{Field: "to", Message: "units must belong to the same measurement type"}
	}
	base := request.Value*from.Scale + from.Offset
	value := (base - to.Offset) / to.Scale
	resultText := formatCalculatorNumber(value) + " " + to.Symbol
	inputText := formatCalculatorNumber(request.Value) + " " + from.Symbol + " → " + to.Symbol
	historyID, err := a.recordCalculatorHistory(ctx, "unit", inputText, resultText, true)
	if err != nil {
		return CalculatorResult{}, err
	}
	return CalculatorResult{Result: value, ResultText: resultText, Approximate: true, HistoryID: historyID}, nil
}

func (a *Service) CalculateDateContext(ctx context.Context, request DateCalculationRequest) (DateCalculationResult, error) {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(request.StartDate))
	if err != nil {
		return DateCalculationResult{}, &ValidationError{Field: "start_date", Message: "start date must use YYYY-MM-DD"}
	}
	var result DateCalculationResult
	var inputText string
	switch request.Operation {
	case "difference":
		end, parseErr := time.Parse("2006-01-02", strings.TrimSpace(request.EndDate))
		if parseErr != nil {
			return DateCalculationResult{}, &ValidationError{Field: "end_date", Message: "end date must use YYYY-MM-DD"}
		}
		days := int(end.Sub(start).Hours() / 24)
		result.Days = days
		result.ResultText = fmt.Sprintf("%d days", days)
		inputText = request.StartDate + " → " + request.EndDate
	case "add", "subtract":
		if request.Amount < 0 || request.Amount > 100000 {
			return DateCalculationResult{}, &ValidationError{Field: "amount", Message: "date amount must be between 0 and 100000"}
		}
		amount := request.Amount
		if request.Operation == "subtract" {
			amount = -amount
		}
		var calculated time.Time
		switch request.Unit {
		case "days":
			calculated = start.AddDate(0, 0, amount)
		case "weeks":
			calculated = start.AddDate(0, 0, amount*7)
		case "months":
			calculated = start.AddDate(0, amount, 0)
		case "years":
			calculated = start.AddDate(amount, 0, 0)
		default:
			return DateCalculationResult{}, &ValidationError{Field: "unit", Message: "date unit must be days, weeks, months, or years"}
		}
		result.Date = calculated.Format("2006-01-02")
		result.ResultText = result.Date
		inputText = fmt.Sprintf("%s %d %s from %s", request.Operation, request.Amount, request.Unit, request.StartDate)
	default:
		return DateCalculationResult{}, &ValidationError{Field: "operation", Message: "date operation must be difference, add, or subtract"}
	}
	historyID, err := a.recordCalculatorHistory(ctx, "date", inputText, result.ResultText, false)
	if err != nil {
		return DateCalculationResult{}, err
	}
	result.HistoryID = historyID
	return result, nil
}

func (a *Service) ListCalculatorHistoryContext(ctx context.Context, limit int) ([]CalculatorHistoryItem, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, &ValidationError{Field: "limit", Message: "history limit must be between 1 and 500"}
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, mode, input_text, result_text, is_approximate, created_at
		FROM calculator_history ORDER BY created_at DESC, id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list calculator history: %w", err)
	}
	defer rows.Close()
	items := make([]CalculatorHistoryItem, 0)
	for rows.Next() {
		var item CalculatorHistoryItem
		var approximate int
		if err := rows.Scan(&item.ID, &item.Mode, &item.InputText, &item.ResultText, &approximate, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan calculator history: %w", err)
		}
		item.Approximate = approximate != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *Service) DeleteCalculatorHistoryItemContext(ctx context.Context, id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "history ID must be positive"}
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM calculator_history WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete calculator history: %w", err)
	}
	return requireSingleMutation(result, "delete calculator history", "calculator history item", false)
}

func (a *Service) ClearCalculatorHistoryContext(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM calculator_history`)
	if err != nil {
		return fmt.Errorf("clear calculator history: %w", err)
	}
	return nil
}

func (a *Service) recordCalculatorHistory(ctx context.Context, mode, input, result string, approximate bool) (int64, error) {
	transaction, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin calculator history transaction: %w", err)
	}
	defer transaction.Rollback()
	inserted, err := transaction.ExecContext(ctx, `
		INSERT INTO calculator_history (mode, input_text, result_text, is_approximate)
		VALUES (?, ?, ?, ?)
	`, mode, input, result, boolDatabaseValue(approximate))
	if err != nil {
		return 0, fmt.Errorf("record calculator history: %w", err)
	}
	id, err := inserted.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read calculator history ID: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		DELETE FROM calculator_history WHERE id IN (
			SELECT id FROM calculator_history ORDER BY created_at DESC, id DESC LIMIT -1 OFFSET 500
		)
	`); err != nil {
		return 0, fmt.Errorf("prune calculator history: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit calculator history: %w", err)
	}
	return id, nil
}

func formatCalculatorNumber(value float64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'g', 15, 64)
}

type calculationParser struct {
	input       string
	position    int
	answer      float64
	approximate bool
}

func (p *calculationParser) parse() (float64, error) {
	value, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.position != len(p.input) {
		return 0, p.errorf("unexpected character %q", p.input[p.position])
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, p.errorf("result is not a finite number")
	}
	return value, nil
}

func (p *calculationParser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.take('+') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
		} else if p.take('-') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
		} else {
			return left, nil
		}
	}
}

func (p *calculationParser) parseTerm() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.take('*') {
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			left *= right
		} else if p.take('/') {
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, p.errorf("division by zero")
			}
			left /= right
		} else {
			return left, nil
		}
	}
}

func (p *calculationParser) parseUnary() (float64, error) {
	p.skipSpace()
	if p.take('+') {
		return p.parseUnary()
	}
	if p.take('-') {
		value, err := p.parseUnary()
		return -value, err
	}
	return p.parsePower()
}

func (p *calculationParser) parsePower() (float64, error) {
	left, err := p.parsePostfix()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.take('^') {
		right, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		left = math.Pow(left, right)
		p.approximate = true
	}
	return left, nil
}

func (p *calculationParser) parsePostfix() (float64, error) {
	value, err := p.parsePrimary()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if !p.take('%') {
			return value, nil
		}
		value /= 100
	}
}

func (p *calculationParser) parsePrimary() (float64, error) {
	p.skipSpace()
	if p.take('(') {
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if !p.take(')') {
			return 0, p.errorf("expected closing parenthesis")
		}
		return value, nil
	}
	if p.position < len(p.input) && (unicode.IsLetter(rune(p.input[p.position]))) {
		identifier := strings.ToLower(p.readIdentifier())
		switch identifier {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		case "ans":
			return p.answer, nil
		}
		p.skipSpace()
		if !p.take('(') {
			return 0, p.errorf("unknown value %q", identifier)
		}
		argument, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if !p.take(')') {
			return 0, p.errorf("expected closing parenthesis")
		}
		p.approximate = true
		switch identifier {
		case "sqrt":
			if argument < 0 {
				return 0, p.errorf("square root requires a non-negative value")
			}
			return math.Sqrt(argument), nil
		case "sin":
			return math.Sin(argument), nil
		case "cos":
			return math.Cos(argument), nil
		case "tan":
			return math.Tan(argument), nil
		case "abs":
			return math.Abs(argument), nil
		case "ln":
			if argument <= 0 {
				return 0, p.errorf("natural logarithm requires a positive value")
			}
			return math.Log(argument), nil
		case "log":
			if argument <= 0 {
				return 0, p.errorf("logarithm requires a positive value")
			}
			return math.Log10(argument), nil
		default:
			return 0, p.errorf("unknown function %q", identifier)
		}
	}
	return p.readNumber()
}

func (p *calculationParser) readNumber() (float64, error) {
	start := p.position
	dotSeen := false
	for p.position < len(p.input) {
		character := p.input[p.position]
		if character >= '0' && character <= '9' {
			p.position++
			continue
		}
		if character == '.' && !dotSeen {
			dotSeen = true
			p.position++
			continue
		}
		break
	}
	if start == p.position {
		return 0, p.errorf("expected a number")
	}
	value, err := strconv.ParseFloat(p.input[start:p.position], 64)
	if err != nil {
		return 0, p.errorf("invalid number")
	}
	return value, nil
}

func (p *calculationParser) readIdentifier() string {
	start := p.position
	for p.position < len(p.input) {
		character := rune(p.input[p.position])
		if !unicode.IsLetter(character) {
			break
		}
		p.position++
	}
	return p.input[start:p.position]
}

func (p *calculationParser) skipSpace() {
	for p.position < len(p.input) && unicode.IsSpace(rune(p.input[p.position])) {
		p.position++
	}
}

func (p *calculationParser) take(character byte) bool {
	if p.position < len(p.input) && p.input[p.position] == character {
		p.position++
		return true
	}
	return false
}

func (p *calculationParser) errorf(format string, arguments ...any) error {
	return fmt.Errorf(format+" at position %d", append(arguments, p.position+1)...)
}
