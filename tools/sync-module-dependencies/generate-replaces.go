package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

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

	projectReplacesPath := filepath.Join(projectRoot, "dependency-replacements.yaml")
	absReplacesPath, err := filepath.Abs(projectReplacesPath)
	if err != nil {
		log.Fatalf("Failed to resolve path to dependency-replacements.yaml: %v", err)
	}
	// Templates are in the script directory, not the project root
	baseDir, err := filepath.Abs(scriptDir)
	if err != nil {
		log.Fatalf("Failed to resolve script directory: %v", err)
	}

	var projectReplaces types.ProjectReplaces
	data, err := os.ReadFile(absReplacesPath)
	if err != nil {
		log.Fatalf("Failed to read dependency-replacements.yaml: %v", err)
	}

	if err := yaml.Unmarshal(data, &projectReplaces); err != nil {
		log.Fatalf("Failed to parse dependency-replacements.yaml: %v", err)
	}

	normalizeComments(projectReplaces.Replaces)
	setDefaultScope(projectReplaces.Replaces)

	for _, module := range projectReplaces.Modules {
		moduleReplaces := filterByScope(projectReplaces.Replaces, module.Name)

		var templatePath string
		switch module.FileType {
		case "mod":
			templatePath = filepath.Join(baseDir, "replaces-mod.tpl")
		default:
			log.Fatalf("Unknown file_type %q for module %q (expected 'mod')", module.FileType, module.Name)
		}

		if err := generateOutput(templatePath, module.OutputFile, moduleReplaces); err != nil {
			log.Fatalf("Failed to generate output for module %q: %v", module.Name, err)
		}
	}
}

func normalizeComments(entries []types.ReplaceEntry) {
	for i := range entries {
		entries[i].Comment = strings.ReplaceAll(entries[i].Comment, "\n", " ")
		entries[i].Comment = strings.TrimSpace(entries[i].Comment)
	}
}

func setDefaultScope(entries []types.ReplaceEntry) {
	for i := range entries {
		if len(entries[i].Scope) == 0 {
			entries[i].Scope = []string{"alloy"}
		}
	}
}

func filterByScope(entries []types.ReplaceEntry, scope string) []types.ReplaceEntry {
	var filtered []types.ReplaceEntry
	for _, entry := range entries {
		for _, s := range entry.Scope {
			if s == scope {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered
}

func generateOutput(templatePath, outputPath string, data []types.ReplaceEntry) error {
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templatePath, err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	return nil
}
