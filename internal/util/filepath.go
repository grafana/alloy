package util

import (
	"os"
	"path/filepath"
)

// ExtractDirPath removes the file part of a path if it exists.
func ExtractDirPath(p string) (string, error) {
	info, err := os.Stat(p)

	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return filepath.Dir(p), nil
	}

	return p, nil
}
