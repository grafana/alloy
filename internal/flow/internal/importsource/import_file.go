package importsource

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/local/file"
	"github.com/grafana/river/vm"
)

// ImportFile imports a module from a file via the local.file component.
type ImportFile struct {
	fileComponent *file.Component
	arguments     component.Arguments
	managedOpts   component.Options
	eval          *vm.Evaluator
}

var _ ImportSource = (*ImportFile)(nil)

func NewImportFile(managedOpts component.Options, eval *vm.Evaluator, onContentChange func(string)) *ImportFile {
	opts := managedOpts
	opts.OnStateChange = func(e component.Exports) {
		onContentChange(e.(file.Exports).Content.Value)
	}
	return &ImportFile{
		managedOpts: opts,
		eval:        eval,
	}
}

// FileArguments holds values which are used to configure the local.file component.
type FileArguments struct {
	// Filename indicates the file to watch.
	Filename string `river:"filename,attr"`
	// Type indicates how to detect changes to the file.
	Type file.Detector `river:"detector,attr,optional"`
	// PollFrequency determines the frequency to check for changes when Type is Poll.
	PollFrequency time.Duration `river:"poll_frequency,attr,optional"`
}

var DefaultFileArguments = FileArguments{
	Type:          file.DetectorFSNotify,
	PollFrequency: time.Minute,
}

type importFileConfigBlock struct {
	LocalFileArguments FileArguments `river:",squash"`
}

// SetToDefault implements river.Defaulter.
func (a *importFileConfigBlock) SetToDefault() {
	a.LocalFileArguments = DefaultFileArguments
}

func (im *ImportFile) Evaluate(scope *vm.Scope) error {
	var arguments importFileConfigBlock
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding River: %w", err)
	}
	if im.fileComponent == nil {
		var err error
		im.fileComponent, err = file.New(im.managedOpts, file.Arguments{
			Filename:      arguments.LocalFileArguments.Filename,
			Type:          arguments.LocalFileArguments.Type,
			PollFrequency: arguments.LocalFileArguments.PollFrequency,
			// isSecret is only used for exported values; modules are not exported
			IsSecret: false,
		})
		if err != nil {
			return fmt.Errorf("creating file component: %w", err)
		}
		im.arguments = arguments
	}

	if reflect.DeepEqual(im.arguments, arguments) {
		return nil
	}

	// Update the existing managed component
	if err := im.fileComponent.Update(arguments); err != nil {
		return fmt.Errorf("updating component: %w", err)
	}
	im.arguments = arguments
	return nil
}

func (im *ImportFile) Run(ctx context.Context) error {
	return im.fileComponent.Run(ctx)
}

// CurrentHealth returns the health of the file component.
func (im *ImportFile) CurrentHealth() component.Health {
	return im.fileComponent.CurrentHealth()
}
