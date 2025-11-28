package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/grafana/replace-generator/internal/helpers"
	"github.com/grafana/replace-generator/internal/types"
)

func GenerateReplaces(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces) {
	normalizeComments(projectReplaces.Replaces)

	// Group modules by file type and generate one output file per file type
	fileTypesSeen := make(map[types.FileType]bool)
	for _, module := range projectReplaces.Modules {
		if !fileTypesSeen[module.FileType] {
			if err := generateReplacesForFileType(fileHelper, projectReplaces, module.FileType); err != nil {
				log.Fatalf("Failed to generate replaces for file type %q: %v", module.FileType, err)
			}
			fileTypesSeen[module.FileType] = true
		}
	}
}

// Removes unnecessary newlines and whitespaces from comment metadata
func normalizeComments(entries []types.ReplaceEntry) {
	for i := range entries {
		entries[i].Comment = strings.ReplaceAll(entries[i].Comment, "\n", " ")
		entries[i].Comment = strings.TrimSpace(entries[i].Comment)
	}
}

// Generates the .txt files that contain the replace directives formatted into their corresponding template
// For file type "mod" we will output mod-replaces.txt generated from the replaces-mod.tpl
func generateReplacesForFileType(dirs *helpers.FileHelper, projectReplaces *types.ProjectReplaces, fileType types.FileType) error {
	templatePath := dirs.TemplatePath(fileType)
	outputPath := dirs.OutputPath(fileType)

	err := generateFromTemplate(templatePath, projectReplaces.Replaces, outputPath)
	if err != nil {
		return fmt.Errorf("could not execute template generation: %w", err)
	}

	return nil
}

func generateFromTemplate(templatePath string, entries []types.ReplaceEntry, outputPath string) error {
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("could not read template %s: %w", templatePath, err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(tmplContent))

	if err != nil {
		return fmt.Errorf("could not parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, entries); err != nil {
		return fmt.Errorf("could not generate template: %w", err)
	}

	// Use 0644 permissions (read/write for owner, read for others)
	// On Windows, these permissions are ignored but the file will still be created correctly
	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("could not write output file: %w", err)
	}

	return nil
}
