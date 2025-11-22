package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
		if err := applyReplacesToModule(scriptDir, projectRoot, module); err != nil {
			log.Fatalf("Failed to apply replaces to module %q: %v", module.Name, err)
		}
	}
}

func applyReplacesToModule(scriptDir string, projectRoot string, module types.Module) error {
	targetPath := filepath.Join(projectRoot, module.Path)

	replacesPath := filepath.Join(scriptDir, module.OutputFile)

	replacesContent, err := os.ReadFile(replacesPath)
	if err != nil {
		return fmt.Errorf("read replaces file %s: %w", replacesPath, err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read target file %s: %w", targetPath, err)
	}

	var startMarker, endMarker string
	switch module.FileType {
	case "mod":
		// Go mod files use // comments
		startMarker = "// BEGIN GENERATED REPLACES - DO NOT EDIT"
		endMarker = "// END GENERATED REPLACES"
	default:
		return fmt.Errorf("unknown file_type %q for module %q (expected 'mod')", module.FileType, module.Name)
	}

	contentStr := string(content)
	hasMarkers := hasMarkers(contentStr, startMarker, endMarker)

	var newContent string
	if hasMarkers {
		newContent = removeBetweenMarkers(contentStr, startMarker, endMarker)
	} else {
		// No markers found, use original content
		newContent = contentStr
	}

	newContent = strings.TrimRight(newContent, "\n") + "\n" + string(replacesContent)

	if err := os.WriteFile(targetPath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("write target file %s: %w", targetPath, err)
	}

	log.Printf("✅ Updated %s", targetPath)
	return nil
}

func removeBetweenMarkers(content, startMarker, endMarker string) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	inMarkerBlock := false

	startMarkerTrimmed := strings.TrimSpace(startMarker)
	endMarkerTrimmed := strings.TrimSpace(endMarker)

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if !inMarkerBlock && strings.HasPrefix(trimmedLine, startMarkerTrimmed) {
			inMarkerBlock = true
			continue // Skip the start marker line
		}

		if inMarkerBlock && trimmedLine == endMarkerTrimmed {
			inMarkerBlock = false
			continue // Skip the end marker line
		}

		if !inMarkerBlock {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Warning: error scanning content: %v", err)
	}

	return result.String()
}

func hasMarkers(content, startMarker, endMarker string) bool {
	startMarkerTrimmed := strings.TrimSpace(startMarker)
	endMarkerTrimmed := strings.TrimSpace(endMarker)

	hasStart := false
	hasEnd := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		trimmedLine := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmedLine, startMarkerTrimmed) {
			hasStart = true
		}
		if trimmedLine == endMarkerTrimmed {
			hasEnd = true
		}
	}

	return hasStart && hasEnd
}
