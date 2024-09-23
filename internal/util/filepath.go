package util

import "path/filepath"

// ExtractDirPath removes the file part of a path if it exists.
func ExtractDirPath(p string) string {
	// If the base of the path has an extension, it's likely a file.
	if filepath.Ext(filepath.Base(p)) != "" {
		return filepath.Dir(p)
	}
	// Otherwise, return the path as is.
	return p
}
