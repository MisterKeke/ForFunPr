package backend

type fileExplorerShell interface {
	OpenFile(path string) error
	RecycleFile(path string) error
}
