package backend

import "fmt"

type ClipboardState struct {
	Settings  ClipboardSettings `json:"settings"`
	Supported bool              `json:"supported"`
	Running   bool              `json:"running"`
}

func (a *App) GetClipboardState() (ClipboardState, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ClipboardState{}, err
	}
	defer done()
	settings, err := service.GetClipboardSettingsContext(ctx)
	if err != nil {
		return ClipboardState{}, err
	}
	return ClipboardState{Settings: settings, Supported: a.clipboard != nil && a.clipboard.Supported(), Running: a.clipboard != nil && a.clipboard.Running()}, nil
}

func (a *App) UpdateClipboardSettings(settings ClipboardSettings) (ClipboardState, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ClipboardState{}, err
	}
	defer done()
	if settings.CollectionEnabled && (a.clipboard == nil || !a.clipboard.Supported()) {
		return ClipboardState{}, fmt.Errorf("clipboard history is currently supported only on Windows")
	}
	updated, err := service.UpdateClipboardSettingsContext(ctx, settings)
	if err != nil {
		return ClipboardState{}, err
	}
	if updated.CollectionEnabled {
		err = a.clipboard.Start(service.OperationContext(), func(value string) { recordClipboardWithBackoff(service, value) })
		if err != nil {
			service.SetCapability("clipboard", false, false, true, "Clipboard integration could not be started.")
			updated.CollectionEnabled = false
			_, _ = service.UpdateClipboardSettingsContext(ctx, updated)
			return ClipboardState{}, err
		}
		service.SetCapability("clipboard", true, true, true, "")
	} else if a.clipboard != nil {
		a.clipboard.Stop()
		service.SetCapability("clipboard", a.clipboard.Supported(), false, true, "")
	}
	return ClipboardState{Settings: updated, Supported: a.clipboard.Supported(), Running: a.clipboard.Running()}, nil
}

func (a *App) ListClipboardItems(filter ClipboardListFilter) (ClipboardListResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ClipboardListResult{}, err
	}
	defer done()
	return service.ListClipboardItemsContext(ctx, filter)
}

func (a *App) SetClipboardItemPinned(id int, pinned bool) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.SetClipboardItemPinnedContext(ctx, id, pinned)
}

func (a *App) RestoreClipboardItem(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	item, err := service.GetClipboardItemContext(ctx, id)
	if err != nil {
		return err
	}
	if a.clipboard == nil || !a.clipboard.Supported() {
		return fmt.Errorf("clipboard access is unavailable")
	}
	return a.clipboard.SetText(item.Content)
}

func (a *App) DeleteClipboardItem(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteClipboardItemContext(ctx, id)
}

func (a *App) ClearClipboardHistory(keepPinned bool) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.ClearClipboardHistoryContext(ctx, keepPinned)
}
