//go:build windows

package util

import (
	"os/exec"

	"github.com/stretchr/testify/assert"
)

// GetACL returns the output of `icacls <path>`, which lists the ACEs (access
// control entries) and owner for the given file or directory.
func GetACL(path string) (string, error) {
	out, err := exec.Command("icacls", path).CombinedOutput()
	return string(out), err
}

// AssertACLContains runs `icacls <path>` and asserts that each of the expected
// substrings appears in its output. Each substring is typically a single ACE,
// e.g. `NT AUTHORITY\SYSTEM:(OI)(CI)(F)`. We match on substrings rather than the
// full output because icacls formatting can vary across Windows builds.
func AssertACLContains(c *assert.CollectT, path string, want ...string) {
	out, err := GetACL(path)
	assert.NoError(c, err, "icacls %q should succeed", path)
	if err != nil {
		return
	}
	for _, w := range want {
		assert.Contains(c, out, w, "ACL for %q missing entry %q; got:\n%s", path, w, out)
	}
}
