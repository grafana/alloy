package importsource

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/vm"
)

type SourceType int

const (
	File SourceType = iota
	String
	Git
	HTTP
)

const (
	BlockImportFile   = "import.file"
	BlockImportString = "import.string"
	BlockImportHTTP   = "import.http"
	BlockImportGit    = "import.git"
)

const ModulePath = "module_path"

// ImportSource retrieves a module from a source.
type ImportSource interface {
	// Evaluate updates the arguments provided via the Alloy block.
	Evaluate(scope *vm.Scope) error
	// Run the underlying source to be updated when the content changes.
	Run(ctx context.Context) error
	// CurrentHealth returns the current Health status of the running source.
	CurrentHealth() component.Health
	// Update evaluator
	SetEval(eval *vm.Evaluator)
	// ModulePath is the path where the module is stored locally.
	ModulePath() string
}

// NewImportSource creates a new ImportSource depending on the type.
// onContentChange is used by the source when it receives new content.
func NewImportSource(sourceType SourceType, managedOpts component.Options, eval *vm.Evaluator, onContentChange func(map[string]string)) ImportSource {
	switch sourceType {
	case File:
		return NewImportFile(managedOpts, eval, onContentChange)
	case String:
		return NewImportString(eval, onContentChange)
	case HTTP:
		return NewImportHTTP(managedOpts, eval, onContentChange)
	case Git:
		return NewImportGit(managedOpts, eval, onContentChange)
	}
	panic(fmt.Errorf("unsupported source type: %v", sourceType))
}

// GetSourceType returns a SourceType matching a source name.
func GetSourceType(fullName string) SourceType {
	switch fullName {
	case BlockImportFile:
		return File
	case BlockImportString:
		return String
	case BlockImportHTTP:
		return HTTP
	case BlockImportGit:
		return Git
	}
	panic(fmt.Errorf("name does not map to a known source type: %v", fullName))
}
