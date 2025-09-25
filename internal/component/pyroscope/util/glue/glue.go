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
	Impl GenericComponent[ARGS]
}

func (c *GenericComponentGlue[ARGS]) Run(ctx context.Context) error {
	return c.Impl.Run(ctx)
}

func (c *GenericComponentGlue[ARGS]) Update(args component.Arguments) error {
	ga := args.(ARGS)
	return c.Impl.Update(ga)
}
