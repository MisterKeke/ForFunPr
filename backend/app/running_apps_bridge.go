package backend

import (
	"errors"

	"something/backend/runningapps"
)

func (a *App) ListRunningApps() ([]RunningApp, error) {
	_, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()

	if a.runningApps == nil {
		return nil, errors.New("Running apps integration is unavailable.")
	}
	if !a.runningApps.Supported() {
		return nil, runningapps.ErrUnsupported
	}
	return a.runningApps.List(ctx)
}

func (a *App) ActivateRunningApp(windowID string) error {
	_, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()

	if a.runningApps == nil {
		return errors.New("Running apps integration is unavailable.")
	}
	if !a.runningApps.Supported() {
		return runningapps.ErrUnsupported
	}
	return a.runningApps.Activate(ctx, windowID)
}
