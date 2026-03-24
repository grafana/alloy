package types

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type FileType string

const (
	FileTypeMod FileType = "mod"
	FileTypeOCB FileType = "ocb"
)

func (ft FileType) String() string {
	return string(ft)
}

func (ft *FileType) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}

	switch FileType(s) {
	case FileTypeMod:
		*ft = FileTypeMod
		return nil
	case FileTypeOCB:
		*ft = FileTypeOCB
		return nil
	default:
		return fmt.Errorf("invalid Module.file_type %q (expected %q or %q)", s, FileTypeMod, FileTypeOCB)
	}
}

// ReplaceEntry represents a single replace directive for a Go module dependency.
type ReplaceEntry struct {
	Comment     string `yaml:"comment"`
	Dependency  string `yaml:"dependency"`
	Replacement string `yaml:"replacement"`
}

// Module represents a Go module that needs replace directives applied.
type Module struct {
	Name     string   `yaml:"name"`
	Path     string   `yaml:"path"`
	FileType FileType `yaml:"file_type"`
}

// ProjectReplaces is the root structure of the dependency-replacements.yaml file.
type ProjectReplaces struct {
	Modules  []Module       `yaml:"modules"`
	Replaces []ReplaceEntry `yaml:"replaces"`
}
