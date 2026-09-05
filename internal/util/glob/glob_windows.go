//go:build windows || darwin

package glob

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// caseInsensitiveGlobber implements Globber with case-insensitive matching,
// appropriate for Windows and macOS file systems.
type caseInsensitiveGlobber struct{}

// NewGlobber creates a new Globber appropriate for the current platform.
// On Windows and macOS, this returns a case-insensitive globber.
func NewGlobber() Globber {
	return &caseInsensitiveGlobber{}
}

func (g *caseInsensitiveGlobber) FilepathGlob(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern, doublestar.WithCaseInsensitive())
}

func (g *caseInsensitiveGlobber) PathMatch(pattern, path string) (bool, error) {
	return doublestar.PathMatch(strings.ToLower(pattern), strings.ToLower(path))
}
