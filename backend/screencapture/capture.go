package screencapture

import "image"

type Capturer interface {
	Supported() bool
	Capture(mode string) (image.Image, error)
}
