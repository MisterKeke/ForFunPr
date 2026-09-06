package ocr

import "context"

type Engine interface {
	Supported() bool
	Recognize(context.Context, string) (string, string, error)
}
