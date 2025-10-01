package internal

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// PackageFromPath tries to extract go package located at path.
// I will only try to read *.go files (excluding tests) and asumes that
// first package found it the only package in the directory.
func PackageFromPath(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to read files at %s: %w", path, err)
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		// We skip directories, non go files and test files.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, 0)
		if err != nil {
			return "", fmt.Errorf("failed to parse file: %w", err)
		}

		return file.Name.String(), nil
	}

	return "", fmt.Errorf("no go package fount at: %s", path)
}
