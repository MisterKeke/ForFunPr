package launcher

import "context"

// RepositoryOpener is the narrow native boundary for opening a registered
// repository. Callers resolve and validate all paths before invoking it.
type RepositoryOpener interface {
	Supported() bool
	OpenFolder(ctx context.Context, path string) error
	OpenEditor(ctx context.Context, executablePath string, repositoryPath string) error
}
