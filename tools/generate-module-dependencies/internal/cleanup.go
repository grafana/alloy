package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/replace-generator/internal/helpers"
)

func main() {
	fileHelper := helpers.GetFileHelper()

	if err := cleanupOutputFiles(fileHelper.ScriptDir); err != nil {
		log.Fatalf("Failed to cleanup output files: %v", err)
	}
}

// The previous steps in the generation pipeline output some temporary .txt files, cleanupOutputFiles removes these files
func cleanupOutputFiles(scriptDir string) error {
	entries, err := os.ReadDir(scriptDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".txt") {
			continue
		}

		filePath := filepath.Join(scriptDir, name)
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to remove %s: %v", name, err)
			continue
		}
	}

	return nil
}
