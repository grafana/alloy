package importsource

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/vm"
)

// ImportString imports a module from a string.
type ImportString struct {
	arguments       component.Arguments
	eval            *vm.Evaluator
	onContentChange func(string)
}

var _ ImportSource = (*ImportString)(nil)

func NewImportString(eval *vm.Evaluator, onContentChange func(string)) *ImportString {
	return &ImportString{
		eval:            eval,
		onContentChange: onContentChange,
	}
}

type importStringConfigBlock struct {
	Content rivertypes.OptionalSecret `river:"content,attr"`
}

func (im *ImportString) Evaluate(scope *vm.Scope) error {
	var arguments importStringConfigBlock
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding River: %w", err)
	}

	if reflect.DeepEqual(im.arguments, arguments) {
		return nil
	}
	im.arguments = arguments

	// notifies that the content has changed
	im.onContentChange(arguments.Content.Value)

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
