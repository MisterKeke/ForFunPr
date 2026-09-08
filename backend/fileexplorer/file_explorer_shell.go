package fileexplorer

type fileExplorerShell interface {
	Supported() bool
	OpenFile(path string) error
	RecycleFile(path string) error
}
