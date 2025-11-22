package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/grafana/replace-generator/types"
	"gopkg.in/yaml.v3"
)

func main() {
	scriptDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot, err := filepath.Abs(filepath.Join(scriptDir, "..", ".."))
	if err != nil {
		log.Fatalf("Failed to resolve project root: %v", err)
	}

	replacesPath := filepath.Join(projectRoot, "dependency-replacements.yaml")
	var projectReplaces types.ProjectReplaces
	data, err := os.ReadFile(replacesPath)
	if err != nil {
		log.Fatalf("Failed to read dependency-replacements.yaml: %v", err)
	}

	if err := yaml.Unmarshal(data, &projectReplaces); err != nil {
		log.Fatalf("Failed to parse dependency-replacements.yaml: %v", err)
	}

	for _, module := range projectReplaces.Modules {
		moduleDir := filepath.Join(projectRoot, filepath.Dir(module.Path))
		abs, err := filepath.Abs(moduleDir)
		if err != nil {
			log.Fatalf("Failed to resolve path %s: %v", moduleDir, err)
		}

		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = abs
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Printf("Running go mod tidy in %s (module: %s)\n", abs, module.Name)
		if err := cmd.Run(); err != nil {
			log.Fatalf("go mod tidy failed in %s: %v", abs, err)
		}
	}
}
