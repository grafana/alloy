package internal

import (
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/grafana/alloy/tools/generate/moduledeps/internal/helpers"
	"github.com/grafana/alloy/tools/generate/moduledeps/internal/types"
)

func GenerateReplaces(fileHelper *helpers.FileHelper, projectReplaces *types.ProjectReplaces) map[string]*string {
	normalizeComments(projectReplaces.Replaces)
	replaceTxtByMod := make(map[string]*string)

	for _, module := range projectReplaces.Modules {
		str, err := generateReplacesForFileType(fileHelper, projectReplaces, module.FileType)
		if err != nil {
			log.Fatalf("Failed to generate replaces for module %q: %v", module.Name, err)
		}

		replaceTxtByMod[module.Name] = str
	}

	return replaceTxtByMod
}

func normalizeComments(entries []types.ReplaceEntry) {
	for i := range entries {
		entries[i].Comment = strings.ReplaceAll(entries[i].Comment, "\n", " ")
		entries[i].Comment = strings.TrimSpace(entries[i].Comment)
	}
}

func generateReplacesForFileType(dirs *helpers.FileHelper, projectReplaces *types.ProjectReplaces, fileType types.FileType) (*string, error) {
	tmplContent, err := dirs.Template(fileType)

	if err != nil {
		return nil, fmt.Errorf("could not load template: %w", err)
	}

	str, err := generateFromTemplate(string(fileType), tmplContent, projectReplaces.Replaces)

	if err != nil {
		return nil, fmt.Errorf("could not execute template generation: %w", err)
	}

	return str, nil
}

func generateFromTemplate(name string, tmplContent []byte, entries []types.ReplaceEntry) (*string, error) {
	tmpl, err := template.New(name).Parse(string(tmplContent))

	if err != nil {
		return nil, fmt.Errorf("could not parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, entries); err != nil {
		return nil, fmt.Errorf("could not generate template: %w", err)
	}

	str := buf.String()
	return &str, nil
}
