package launcher

type Launcher interface {
	Launch(path string) error
}
