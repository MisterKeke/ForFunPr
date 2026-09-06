package clipboard

import "context"

type Controller interface {
	Supported() bool
	Running() bool
	Start(context.Context, func(string)) error
	Stop()
	SetText(string) error
}
