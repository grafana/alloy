package internal

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/grafana/replace-generator/internal/helpers"
	"github.com/grafana/replace-generator/internal/types"
)

func ApplyReplaces(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces, modByReplaceStr map[string]*string) {
	for _, module := range projectReplaces.Modules {
		replacesStr := modByReplaceStr[module.Name]
		if err := applyReplacesToModule(fileHelper, module, replacesStr); err != nil {
			log.Fatalf("Failed to apply replaces to module %q: %v", module.Name, err)
		}
		log.Printf("Updated %s", module.Path)
	}
}

func applyReplacesToModule(dirs *helpers.FileHelper, module types.Module, replacesStr *string) error {
	targetPath := dirs.ModuleTargetPath(module.Path)

	targetContent, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read target file %s: %w", targetPath, err)
	}

	startMarker, endMarker, err := getMarkers(module.FileType)
	if err != nil {
		return fmt.Errorf("get markers for file type %q: %w", module.FileType, err)
	}

	newContent, err := upsertGeneratedBlock(string(targetContent), *replacesStr, startMarker, endMarker)
	if err != nil {
		return fmt.Errorf("update generated block: %w", err)
	}

	if err := os.WriteFile(targetPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write target file %s: %w", targetPath, err)
	}

	return nil
}

func getCommentMarker(fileType types.FileType) (string, error) {
	switch fileType {
	case types.FileTypeMod:
		return "//", nil
	case types.FileTypeOCB:
		return "#", nil
	default:
		return "", fmt.Errorf("Unknown file_type %q (expected %q or %q)", fileType, types.FileTypeMod, types.FileTypeOCB)
	}
}

func getMarkers(fileType types.FileType) (startMarker, endMarker string, err error) {
	commentSymbol, err := getCommentMarker(fileType)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY", commentSymbol),
		fmt.Sprintf("%s END GENERATED REPLACES", commentSymbol),
		nil
}

// Upserts the generated block using the markers, or lack thereof, as a guide
func upsertGeneratedBlock(targetContent, replacement, startMarker, endMarker string) (string, error) {
	lineStart := strings.Index(targetContent, startMarker)
	startFound := lineStart != -1

	if !startFound {
		// No start marker: if the end marker exists anywhere, it's invalid.
		if strings.Contains(targetContent, endMarker) {
			return "", fmt.Errorf("found end marker without start marker")
		}

		// Neither start not end marker found, append to the end of the file
		targetContent = strings.TrimRight(targetContent, "\n")
		return targetContent + "\n" + replacement, nil
	}

	// Find the start of the line containing the start marker
	for lineStart > 0 && targetContent[lineStart-1] != '\n' {
		lineStart--
	}

	searchFrom := lineStart + len(startMarker)
	endRel := strings.Index(targetContent[searchFrom:], endMarker)
	if endRel == -1 {
		// Start marker exists without an end marker, which is invalid
		return "", fmt.Errorf("found start marker without end marker")
	}

	lineEnd := searchFrom + endRel + len(endMarker)

	// Find the end of the line containing the end marker (or end of file)
	for lineEnd < len(targetContent) && targetContent[lineEnd] != '\n' {
		lineEnd++
	}
	// Include the newline if present
	if lineEnd < len(targetContent) {
		lineEnd++
	}

	return targetContent[:lineStart] + replacement + targetContent[lineEnd:], nil
}
