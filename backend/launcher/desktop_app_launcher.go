package launcher

type Launcher interface {
	Supported() bool
	Launch(path string) error
}
