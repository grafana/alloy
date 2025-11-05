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
	projectReplacesPath := "dependency-replacements.yaml"

	absReplacesPath, err := filepath.Abs(projectReplacesPath)
	if err != nil {
		log.Fatalf("Failed to resolve path to dependency-replacements.yaml: %v", err)
	}
	baseDir := filepath.Dir(absReplacesPath)

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
			templatePath = filepath.Join(baseDir, "replaces_alloy_gomod.tpl")
		case "yaml":
			templatePath = filepath.Join(baseDir, "replaces_yaml.tpl")
		default:
			log.Fatalf("Unknown file_type %q for module %q (expected 'mod' or 'yaml')", module.FileType, module.Name)
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
			entries[i].Scope = []string{"alloy", "collector", "extension"}
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
