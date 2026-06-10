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
