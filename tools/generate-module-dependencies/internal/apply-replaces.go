package internal

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/grafana/replace-generator/internal/helpers"
	"github.com/grafana/replace-generator/internal/types"
)

func ApplyReplaces(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces) {
	for _, module := range projectReplaces.Modules {
		if err := applyReplacesToModule(fileHelper, module); err != nil {
			log.Fatalf("Failed to apply replaces to module %q: %v", module.Name, err)
		}
		log.Printf("Updated %s", module.Path)
	}
}

func applyReplacesToModule(dirs *helpers.FileHelper, module types.Module) error {
	targetPath := dirs.ModuleTargetPath(module.Path)
	replacesPath := dirs.OutputPath(module.FileType)

	replacesContent, err := os.ReadFile(replacesPath)
	if err != nil {
		return fmt.Errorf("read replaces file %s: %w", replacesPath, err)
	}

	targetContent, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read target file %s: %w", targetPath, err)
	}

	startMarker, endMarker, err := getMarkers(module.FileType)
	if err != nil {
		return fmt.Errorf("get markers for file type %q: %w", module.FileType, err)
	}

	newContent, err := upsertGeneratedBlock(string(targetContent), string(replacesContent), startMarker, endMarker)
	if err != nil {
		return fmt.Errorf("update generated block: %w", err)
	}

	// Use 0644 permissions (read/write for owner, read for others)
	// On Windows, these permissions are ignored but the file will still be created correctly
	if err := os.WriteFile(targetPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write target file %s: %w", targetPath, err)
	}

	return nil
}

func getMarkers(fileType types.FileType) (startMarker, endMarker string, err error) {
	switch fileType {
	case types.FileTypeMod:
		return "// BEGIN GENERATED REPLACES - DO NOT EDIT", "// END GENERATED REPLACES", nil
	default:
		return "", "", fmt.Errorf("unknown file_type %q", fileType)
	}
}

// Upserts the generated block using the markers, or lack thereof, as a guide
func upsertGeneratedBlock(targetContent, replacement, startMarker, endMarker string) (string, error) {
	startIdx := strings.Index(targetContent, startMarker)
	startFound := startIdx != -1

	if !startFound {
		// No start marker: if the end marker exists anywhere, it's invalid.
		if strings.Contains(targetContent, endMarker) {
			return "", fmt.Errorf("found end marker without start marker")
		}

		// Neither start not end marker found, append to the end of the file
		targetContent = strings.TrimRight(targetContent, "\n")
		return targetContent + "\n" + replacement, nil
	}

	// Start found: search end marker after the start
	searchFrom := startIdx + len(startMarker)
	endRel := strings.Index(targetContent[searchFrom:], endMarker)
	if endRel == -1 {
		// Start marker exists without an end marker, which is invalid
		return "", fmt.Errorf("found start marker without end marker")
	}

	endIdx := searchFrom + endRel

	// Compute the end of the line containing the end marker (include trailing newline if present).
	// Handle both \n and \r\n line endings
	endOfMarker := endIdx + len(endMarker)
	remaining := targetContent[endOfMarker:]

	// Check for \r\n first (Windows), then \n (Unix)
	if strings.HasPrefix(remaining, "\r\n") {
		endOfMarker += 2
	} else if strings.HasPrefix(remaining, "\n") {
		endOfMarker += 1
	}

	// Replace [startIdx, endOfMarker) with replacement.
	return targetContent[:startIdx] + replacement + targetContent[endOfMarker:], nil
}
