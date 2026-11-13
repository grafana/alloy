// Package gha writes GitHub Actions workflow outputs.
package gha

import (
	"fmt"
	"os"
	"strings"
)

// SetOutput appends a name/value pair to GITHUB_OUTPUT. It is a no-op when
// GITHUB_OUTPUT is unset, so commands stay usable outside Actions.
func SetOutput(name, value string) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}

	outputFile, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening GITHUB_OUTPUT: %w", err)
	}
	defer outputFile.Close()

	delimiter := "EOF"
	for strings.Contains(value, delimiter) {
		delimiter += "_EOF"
	}

	if _, err := fmt.Fprintf(outputFile, "%s<<%s\n%s\n%s\n", name, delimiter, value, delimiter); err != nil {
		return fmt.Errorf("writing GITHUB_OUTPUT %s: %w", name, err)
	}
	return nil
}
