package glue

import (
	"context"

	"github.com/grafana/alloy/internal/component"
)

type GenericComponent[ARGS any] interface {
	Run(ctx context.Context) error
	Update(args ARGS) error
}

type GenericComponentGlue[ARGS any] struct {
	GenericComponent[ARGS]
}

func (p *GenericComponentGlue[ARGS]) Run(ctx context.Context) error {
	return p.GenericComponent.Run(ctx)
}

func (p *GenericComponentGlue[ARGS]) Update(args component.Arguments) error {
	return p.GenericComponent.Update(args.(ARGS))
}
