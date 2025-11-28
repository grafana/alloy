package helpers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/grafana/replace-generator/internal/types"
	"gopkg.in/yaml.v3"
)

type FileHelper struct {
	// ScriptDir is the directory where the generate-module-dependencies code is located
	ScriptDir string

	// ProjectRoot is the root directory of the Alloy project
	ProjectRoot string

	// ProjectReplacesPath is the absolute path to dependency-replacements.yaml.
	ProjectReplacesPath string
}

func NewFileHelper(pathToDependencyReplacements string, projectRoot string) *FileHelper {
	scriptDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to resolve working directory: %v", err)
	}

	scriptDir, err = filepath.Abs(scriptDir)
	if err != nil {
		log.Fatalf("Failed to resolve script directory: %v", err)
	}

	absReplacesPath, err := filepath.Abs(pathToDependencyReplacements)
	if err != nil {
		log.Fatalf("Failed to resolve dependency-replacements.yaml: %v", err)
	}

	return &FileHelper{
		ScriptDir:           scriptDir,
		ProjectRoot:         projectRoot,
		ProjectReplacesPath: absReplacesPath,
	}
}

func (d *FileHelper) TemplatePath(fileType types.FileType) string {
	var templateName string
	switch fileType {
	case types.FileTypeMod:
		templateName = "replaces-mod.tpl"
	default:
		log.Fatalf("Unknown file_type %q (expected %q)", fileType, types.FileTypeMod)
	}
	return filepath.Join(d.ScriptDir, templateName)
}

func (d *FileHelper) ModuleTargetPath(modulePath string) string {
	return filepath.Join(d.ProjectRoot, modulePath)
}

func (d *FileHelper) ModuleDir(modulePath string) string {
	moduleDir := filepath.Join(d.ProjectRoot, filepath.Dir(modulePath))
	abs, err := filepath.Abs(moduleDir)
	if err != nil {
		log.Fatalf("Failed to resolve module directory %s: %v", moduleDir, err)
	}
	return abs
}

func (d *FileHelper) LoadProjectReplaces() (*types.ProjectReplaces, error) {
	data, err := os.ReadFile(d.ProjectReplacesPath)
	if err != nil {
		return nil, fmt.Errorf("read dependency-replacements.yaml: %w", err)
	}

	var projectReplaces types.ProjectReplaces
	if err := yaml.Unmarshal(data, &projectReplaces); err != nil {
		return nil, fmt.Errorf("parse dependency-replacements.yaml: %w", err)
	}

	return &projectReplaces, nil
}
