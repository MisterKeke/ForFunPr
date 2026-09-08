package screencapture

import (
	"context"
	"errors"
	"image"
)

type Mode string

const (
	ModeScreen Mode = "screen"
	ModeWindow Mode = "window"
	ModeRegion Mode = "region"
)

type Rectangle struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Request struct {
	Mode        Mode       `json:"mode"`
	Region      *Rectangle `json:"region,omitempty"`
	Interactive bool       `json:"interactive"`
}

type Result struct {
	Image          image.Image
	Kind           Mode
	Bounds         Rectangle
	RegionSelected bool
}

var ErrCancelled = errors.New("screen capture cancelled")

type Capturer interface {
	Supported() bool
	WaitUntilReady(context.Context) error
	Capture(context.Context, Request) (Result, error)
}
