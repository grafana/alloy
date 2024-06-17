package vcs

import "fmt"

// DownloadFailedError represents a failure to download a repository.
type DownloadFailedError struct {
	Repository string
	Inner      error
}

// Error returns the error string, denoting the failed repository.
func (err DownloadFailedError) Error() string {
	if err.Inner == nil {
		return fmt.Sprintf("failed to download repository %q", err.Repository)
	}
	return fmt.Sprintf("failed to download repository %q: %s", err.Repository, err.Inner)
}

// Unwrap returns the inner error. It returns nil if there is no inner error.
func (err DownloadFailedError) Unwrap() error { return err.Inner }

// UpdateFailedError represents a failure to update a repository.
type UpdateFailedError struct {
	Repository string
	Inner      error
}

// Error returns the error string, denoting the failed repository.
func (err UpdateFailedError) Error() string {
	if err.Inner == nil {
		return fmt.Sprintf("failed to update repository %q", err.Repository)
	}
	return fmt.Sprintf("failed to update repository %q: %s", err.Repository, err.Inner)
}

// Unwrap returns the inner error. It returns nil if there is no inner error.
func (err UpdateFailedError) Unwrap() error { return err.Inner }

// InvalidRevisionError represents an invalid revision.
type InvalidRevisionError struct {
	Revision string
}

// Error returns the error string, denoting the invalid revision.
func (err InvalidRevisionError) Error() string {
	return fmt.Sprintf("invalid revision \"%s\"", err.Revision)
}
