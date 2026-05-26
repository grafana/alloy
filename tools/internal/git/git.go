package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Root returns the absolute path to the repository's top-level directory.
func Root() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolving git root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil

}

// Tag returns the v*-prefixed tag pointing at HEAD, or an error if HEAD isn't exactly on such a tag.
func Tag() (string, error) {
	out, err := exec.Command("git", "describe", "--match", "v*", "--exact-match").Output()
	if err != nil {
		return "", fmt.Errorf("resolving git tag: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// WorkingTreeDirty reports whether the working tree has uncommitted changes.
func WorkingTreeDirty() (bool, error) {
	out, err := exec.Command("git", "status", "-s").Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// ShortSHA returns the abbreviated commit SHA of HEAD.
func ShortSHA() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
