//go:build windows

package glob

import "github.com/bmatcuk/doublestar/v4"

// windowsGlobber implements Globber with case-insensitive matching,
// appropriate for Windows file systems.
type windowsGlobber struct{}

// NewGlobber creates a new Globber appropriate for the current platform.
// On Windows, this returns a case-insensitive globber.
func NewGlobber() Globber {
	return &windowsGlobber{}
}

func (g *windowsGlobber) FilepathGlob(pattern string) ([]string, error) {
	return doublestar.FilepathGlob(pattern, doublestar.WithCaseInsensitive())
}

func (g *windowsGlobber) PathMatch(pattern, path string) (bool, error) {
	return doublestar.PathMatch(pattern, path)
}
