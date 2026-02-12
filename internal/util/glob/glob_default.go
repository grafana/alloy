//go:build !windows

package glob

import "github.com/bmatcuk/doublestar/v4"

// defaultGlobber implements Globber with default options (case-sensitive).
type defaultGlobber struct{}

// NewGlobber creates a new Globber appropriate for the current platform.
func NewGlobber() Globber {
	return &defaultGlobber{}
}

func (g *defaultGlobber) FilepathGlob(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern)
}

func (g *defaultGlobber) PathMatch(pattern, path string) (bool, error) {
	return doublestar.PathMatch(pattern, path)
}
