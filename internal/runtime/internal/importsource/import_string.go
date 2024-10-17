package importsource

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/vm"
)

// ImportString imports a module from a string.
type ImportString struct {
	arguments       component.Arguments
	eval            *vm.Evaluator
	onContentChange func(map[string]string)
	modulePath      string
}

var _ ImportSource = (*ImportString)(nil)

func NewImportString(eval *vm.Evaluator, onContentChange func(map[string]string)) *ImportString {
	return &ImportString{
		eval:            eval,
		onContentChange: onContentChange,
	}
}

type importStringConfigBlock struct {
	Content alloytypes.OptionalSecret `alloy:"content,attr"`
}

func (im *ImportString) Evaluate(scope *vm.Scope) error {
	var arguments importStringConfigBlock
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	if reflect.DeepEqual(im.arguments, arguments) {
		return nil
	}
	im.arguments = arguments

	im.modulePath, _ = scope.Variables[ModulePath].(string)

	// notifies that the content has changed
	im.onContentChange(map[string]string{"import_string": arguments.Content.Value})

	return nil
}

func (im *ImportString) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// ImportString is always healthy
func (im *ImportString) CurrentHealth() component.Health {
	return component.Health{
		Health: component.HealthTypeHealthy,
	}
}

// Update the evaluator.
func (im *ImportString) SetEval(eval *vm.Evaluator) {
	im.eval = eval
}

func (im *ImportString) ModulePath() string {
	return im.modulePath
}
