package backend

type desktopAppLauncher interface {
	Launch(path string) error
}
