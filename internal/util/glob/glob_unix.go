//go:build !windows && !darwin

package glob

import "github.com/bmatcuk/doublestar/v4"

// caseSensitiveGlobber implements Globber with default options (case-sensitive).
type caseSensitiveGlobber struct{}

// NewGlobber creates a new Globber appropriate for the current platform.
// On case-sensitive platforms (such as Linux), this returns a case-sensitive globber.
func NewGlobber() Globber {
	return &caseSensitiveGlobber{}
}

func (g *caseSensitiveGlobber) FilepathGlob(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}

func (g *caseSensitiveGlobber) PathMatch(pattern, path string) (bool, error) {
	return doublestar.PathMatch(pattern, path)
}
