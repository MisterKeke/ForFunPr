package launcher

// RepositoryOpener is the narrow native boundary for opening a registered
// repository. Callers resolve and validate all paths before invoking it.
type RepositoryOpener interface {
	Supported() bool
	OpenFolder(path string) error
	OpenEditor(executablePath string, repositoryPath string) error
}
