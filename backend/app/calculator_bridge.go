package backend

func (a *App) EvaluateCalculatorExpression(request CalculatorExpressionRequest) (CalculatorResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return CalculatorResult{}, err
	}
	defer done()
	return service.EvaluateCalculatorExpressionContext(ctx, request)
}

func (a *App) ListCalculatorUnits() ([]UnitOption, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListCalculatorUnitsContext(ctx), nil
}

func (a *App) ConvertCalculatorUnit(request UnitConversionRequest) (CalculatorResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return CalculatorResult{}, err
	}
	defer done()
	return service.ConvertCalculatorUnitContext(ctx, request)
}

func (a *App) CalculateDate(request DateCalculationRequest) (DateCalculationResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return DateCalculationResult{}, err
	}
	defer done()
	return service.CalculateDateContext(ctx, request)
}

func (a *App) ListCalculatorHistory(limit int) ([]CalculatorHistoryItem, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListCalculatorHistoryContext(ctx, limit)
}

func (a *App) DeleteCalculatorHistoryItem(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteCalculatorHistoryItemContext(ctx, id)
}

func (a *App) ClearCalculatorHistory() error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.ClearCalculatorHistoryContext(ctx)
}
