package testcomponents

import (
	"context"

	"github.com/grafana/agent/internal/component"
)

// Fake is a fake component instance which invokes fields when its methods are
// called. Fake does not register itself as a component and must be provided in
// a custom registry.
//
// The zero value is ready for use.
type Fake struct {
	RunFunc    func(ctx context.Context) error
	UpdateFunc func(args component.Arguments) error
}

var _ component.Component = (*Fake)(nil)

// Run implements [component.Component]. f.RunFunc will be invoked if it is
// non-nil, otherwise a default implementation is used.
func (f *Fake) Run(ctx context.Context) error {
	if f.RunFunc != nil {
		return f.RunFunc(ctx)
	}

	<-ctx.Done()
	return nil
}

// Update implements [component.Component]. f.UpdateFunc will be invoked if it
// is non-nil, otherwise a default implementation is used.
func (f *Fake) Update(args component.Arguments) error {
	if f.UpdateFunc != nil {
		return f.UpdateFunc(args)
	}

	return nil
}
