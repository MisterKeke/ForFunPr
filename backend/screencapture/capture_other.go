//go:build !windows

package screencapture

import (
	"context"
	"errors"
)

type unsupportedCapturer struct{}

func New() Capturer                                              { return unsupportedCapturer{} }
func (unsupportedCapturer) Supported() bool                      { return false }
func (unsupportedCapturer) WaitUntilReady(context.Context) error { return nil }
func (unsupportedCapturer) Capture(context.Context, Request) (Result, error) {
	return Result{}, errors.New("screen capture is currently supported only on Windows")
}
