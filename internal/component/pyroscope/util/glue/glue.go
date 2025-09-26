package glue

import (
	"context"

	"github.com/grafana/alloy/internal/component"
)

type GenericComponent[ARGS any] interface {
	Run(ctx context.Context) error
	Update(args ARGS) error
}

// GenericComponentGlue is a helper to allow writing alloy components without depending on component.Arguments package
// as a tiny nice bonus, the Update method is type-safe and does not require casting interface{} to ARGS
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
