package glob

// Globber provides file path globbing functionality.
// Platform-specific implementations control options like case sensitivity.
type Globber interface {
	// FilepathGlob returns a list of file paths matching the pattern.
	FilepathGlob(pattern string) ([]string, error)
	// PathMatch returns true if the path matches the pattern.
	PathMatch(pattern, path string) (bool, error)
}
