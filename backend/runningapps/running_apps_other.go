//go:build !windows

package runningapps

import "context"

type unsupportedProvider struct{}

func New() Provider {
	return unsupportedProvider{}
}

func (unsupportedProvider) Supported() bool {
	return false
}

func (unsupportedProvider) List(context.Context) ([]App, error) {
	return nil, ErrUnsupported
}

func (unsupportedProvider) Activate(context.Context, string) error {
	return ErrUnsupported
}
