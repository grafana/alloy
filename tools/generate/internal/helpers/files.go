package helpers

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/tools/generate/internal/types"
	"gopkg.in/yaml.v3"
)

//go:embed replaces-mod.tpl
var templateMod []byte

//go:embed replaces-ocb.tpl
var templateOCB []byte

type FileHelper struct {
	// ProjectRoot is the root directory of the Alloy project
	ProjectRoot string

	// ProjectReplacesPath is the absolute path to dependency-replacements.yaml.
	ProjectReplacesPath string
}

func NewFileHelper(pathToDependencyReplacements string, projectRoot string) (*FileHelper, error) {
	absReplacesPath, err := filepath.Abs(pathToDependencyReplacements)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve %s: %v", pathToDependencyReplacements, err)
	}

	return &FileHelper{
		ProjectRoot:         projectRoot,
		ProjectReplacesPath: absReplacesPath,
	}, nil
}

func (d *FileHelper) Template(fileType types.FileType) ([]byte, error) {
	switch fileType {
	case types.FileTypeMod:
		return templateMod, nil
	case types.FileTypeOCB:
		return templateOCB, nil
	default:
		return nil, fmt.Errorf("unknown file_type %q (expected %q or %q)", fileType, types.FileTypeMod, types.FileTypeOCB)
	}
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
