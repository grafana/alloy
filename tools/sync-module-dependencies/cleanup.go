package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	scriptDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Find all .txt files in the script directory
	entries, err := os.ReadDir(scriptDir)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".txt") {
			filePath := filepath.Join(scriptDir, name)
			if err := os.Remove(filePath); err != nil {
				log.Printf("Warning: failed to remove %s: %v", filePath, err)
				continue
			}
			log.Printf("Removed: %s", name)
		}
	}
}
