package helpers

import (
	"fmt"
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

func NewFileHelper(pathToDependencyReplacements string, projectRoot string) (*FileHelper, error) {
	scriptDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve working directory: %v", err)
	}

	scriptDir, err = filepath.Abs(scriptDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve script directory: %v", err)
	}

	absReplacesPath, err := filepath.Abs(pathToDependencyReplacements)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve %s: %v", pathToDependencyReplacements, err)
	}

	return &FileHelper{
		ScriptDir:           scriptDir,
		ProjectRoot:         projectRoot,
		ProjectReplacesPath: absReplacesPath,
	}, nil
}

func (d *FileHelper) TemplatePath(fileType types.FileType) (string, error) {
	var templateName string
	var err error

	switch fileType {
	case types.FileTypeMod:
		templateName = "replaces-mod.tpl"
	default:
		err = fmt.Errorf("Unknown file_type %q (expected %q)", fileType, types.FileTypeMod)
	}
	return filepath.Join(d.ScriptDir, templateName), err
}

func (d *FileHelper) ModuleTargetPath(modulePath string) string {
	return filepath.Join(d.ProjectRoot, modulePath)
}

func (d *FileHelper) ModuleDir(modulePath string) (string, error) {
	moduleDir := filepath.Join(d.ProjectRoot, filepath.Dir(modulePath))
	abs, err := filepath.Abs(moduleDir)

	if err != nil {
		return "", fmt.Errorf("Failed to resolve module directory %s: %v", moduleDir, err)
	}

	return abs, nil
}

func (d *FileHelper) LoadProjectReplaces() (*types.ProjectReplaces, error) {
	data, err := os.ReadFile(d.ProjectReplacesPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", d.ProjectReplacesPath, err)
	}

	var projectReplaces types.ProjectReplaces
	if err := yaml.Unmarshal(data, &projectReplaces); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", d.ProjectReplacesPath, err)
	}

	return &projectReplaces, nil
}
