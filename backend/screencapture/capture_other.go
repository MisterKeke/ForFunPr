//go:build !windows

package screencapture

import (
	"errors"
	"image"
)

type unsupportedCapturer struct{}

func New() Capturer                         { return unsupportedCapturer{} }
func (unsupportedCapturer) Supported() bool { return false }
func (unsupportedCapturer) Capture(string) (image.Image, error) {
	return nil, errors.New("screen capture is currently supported only on Windows")
}
