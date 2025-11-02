package testservices

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
)

// The Fake service allows injecting custom behavior for interface methods.
type Fake struct {
	DefinitionFunc func() service.Definition
	RunFunc        func(ctx context.Context, host service.Host) error
	UpdateFunc     func(newConfig any) error
	DataFunc       func() any
}

var _ service.Service = (*Fake)(nil)

// Definition implements [service.Service]. If f.DefinitionFunc is non-nil, it
// will be used. Otherwise, a default implementation is used.
func (f *Fake) Definition() service.Definition {
	if f.DefinitionFunc != nil {
		return f.DefinitionFunc()
	}

	return service.Definition{Name: "fake"}
}

// Run implements [service.Service]. If f.RunFunc is non-nil, it will be used.
// Otherwise, a default implementation is used.
func (f *Fake) Run(ctx context.Context, host service.Host) error {
	if f.RunFunc != nil {
		return f.RunFunc(ctx, host)
	}

	<-ctx.Done()
	return nil
}

// Update implements [service.Service]. If f.UpdateFunc is non-nil, it will be
// used. Otherwise, a default implementation is used.
func (f *Fake) Update(newConfig any) error {
	if f.UpdateFunc != nil {
		return f.UpdateFunc(newConfig)
	}

	return nil
}

// Data implements [service.Service]. If f.DataFunc is non-nil, it will be
// used. Otherwise, a default implementation is used.
func (f *Fake) Data() any {
	if f.DataFunc != nil {
		return f.DataFunc()
	}

	return nil
}

func (f *Fake) Exports() component.Exports {
	return nil
}
