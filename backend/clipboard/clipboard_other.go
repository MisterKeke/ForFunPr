//go:build !windows

package clipboard

import (
	"context"
	"errors"
)

type unsupportedController struct{}

func New() Controller { return &unsupportedController{} }

func (*unsupportedController) Supported() bool { return false }
func (*unsupportedController) Running() bool   { return false }
func (*unsupportedController) Start(context.Context, func(string)) error {
	return errors.New("clipboard history is currently supported only on Windows")
}
func (*unsupportedController) Stop() {}
func (*unsupportedController) SetText(string) error {
	return errors.New("clipboard access is currently supported only on Windows")
}
