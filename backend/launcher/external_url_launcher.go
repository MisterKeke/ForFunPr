package launcher

import "github.com/pkg/browser"

// ExternalURLLauncher opens a validated web URL with the user's default browser.
type ExternalURLLauncher interface {
	OpenURL(value string) error
}

type systemExternalURLLauncher struct{}

func NewExternalURLLauncher() ExternalURLLauncher {
	return systemExternalURLLauncher{}
}

func (systemExternalURLLauncher) OpenURL(value string) error {
	return browser.OpenURL(value)
}
