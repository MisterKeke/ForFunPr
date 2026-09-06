//go:build !windows

package ocr

import (
	"context"
	"errors"
)

type unsupportedEngine struct{}

func New() Engine                         { return unsupportedEngine{} }
func (unsupportedEngine) Supported() bool { return false }
func (unsupportedEngine) Recognize(context.Context, string) (string, string, error) {
	return "", "", errors.New("local OCR is currently supported only on Windows")
}
