package pipelinetest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tempDirVar is the placeholder for the working directory of a test. It
// works in the config and in file paths. In the config, backslashes are
// doubled, so the value is only valid inside a double-quoted string. Do not
// use it inside a raw string in backticks.
const tempDirVar = "{{TEMP_DIR}}"

// SetupSchema describes the work a test needs before Alloy starts.
type SetupSchema struct {
	// Files are written before Alloy starts.
	Files []SetupFileSchema `yaml:"files,omitempty"`
}

// SetupFileSchema describes a file to create. Missing parent directories are
// created.
type SetupFileSchema struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content,omitempty"`
}

// runSetup creates the files of a test and returns the config with the
// placeholders replaced. A backslash in the working directory is doubled in
// the config, so a Windows path stays valid inside an Alloy string.
func runSetup(schema TestSchema, workDir string) (string, error) {
	config := string(schema.Config)

	if workDir == "" {
		usesPlaceholder := strings.Contains(config, tempDirVar)
		for _, file := range schema.Setup.Files {
			usesPlaceholder = usesPlaceholder || strings.Contains(file.Path, tempDirVar)
		}
		if usesPlaceholder {
			return "", errors.New("the test uses " + tempDirVar + ", but has no working directory")
		}
	}

	config = strings.ReplaceAll(config, tempDirVar, strings.ReplaceAll(workDir, `\`, `\\`))

	for _, file := range schema.Setup.Files {
		if file.Path == "" {
			return "", errors.New("setup file requires path")
		}
		path := filepath.FromSlash(strings.ReplaceAll(file.Path, tempDirVar, workDir))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			return "", fmt.Errorf("write setup file %q: %w", path, err)
		}
	}

	return config, nil
}
