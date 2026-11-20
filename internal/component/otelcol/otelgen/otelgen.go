// Package otelgen describes the Alloy wrappers to generate for upstream
// OpenTelemetry Collector components.
package otelgen

import (
	"reflect"

	"github.com/grafana/alloy/internal/featuregate"

	otelcomponent "go.opentelemetry.io/collector/component"
	otelconnector "go.opentelemetry.io/collector/connector"
	otelprocessor "go.opentelemetry.io/collector/processor"
)

// Kind is the component kind, also the otelcol subdirectory and name segment.
type Kind string

const (
	KindProcessor Kind = "processor"
	KindConnector Kind = "connector"
)

// Component describes an Alloy component wrapping an upstream factory. Create
// one with a kind constructor such as Processor.
type Component struct {
	kind       Kind
	name       string
	factory    otelcomponent.Factory
	newFactory any

	componentName string

	stability       featuregate.Stability
	stabilityReason string

	argumentNames map[string]string

	requiredArguments map[string]bool
	argumentTypes     map[string]reflect.Type
	argumentGoNames   map[string]string

	defaultArguments       func(any)
	defaultArgumentsType   reflect.Type
	defaultArgumentsReason string
}

// Option changes how a Component is generated.
type Option func(*Component)

// Processor describes otelcol.processor.<name>.
func Processor(name string, newFactory func() otelprocessor.Factory, opts ...Option) Component {
	c := Component{kind: KindProcessor, name: name, factory: newFactory(), newFactory: newFactory}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Connector describes otelcol.connector.<name>.
func Connector(name string, newFactory func() otelconnector.Factory, opts ...Option) Component {
	c := Component{kind: KindConnector, name: name, factory: newFactory(), newFactory: newFactory}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func (c Component) Kind() Kind { return c.kind }

// Name returns the package name of the component.
func (c Component) Name() string { return c.name }

// ComponentName returns the component name without the otelcol.<kind> prefix.
func (c Component) ComponentName() string {
	if c.componentName != "" {
		return c.componentName
	}
	return c.factory.Type().String()
}

func (c Component) Factory() otelcomponent.Factory { return c.factory }

// NewFactory returns the upstream NewFactory function the component was
// described with.
func (c Component) NewFactory() any { return c.newFactory }

// Stability returns the stability set with OverrideStability, or
// StabilityUndefined when none is set.
func (c Component) Stability() featuregate.Stability { return c.stability }

// StabilityReason returns the reason given to OverrideStability.
func (c Component) StabilityReason() string { return c.stabilityReason }

// ArgumentNames returns the argument names set with OverrideArgumentName,
// keyed by upstream mapstructure path.
func (c Component) ArgumentNames() map[string]string { return c.argumentNames }

// ArgumentTypes returns the types set with OverrideArgumentType, keyed by
// upstream mapstructure path.
func (c Component) ArgumentTypes() map[string]reflect.Type { return c.argumentTypes }

// ArgumentGoNames returns the field names set with OverrideArgumentGoName,
// keyed by upstream mapstructure path.
func (c Component) ArgumentGoNames() map[string]string { return c.argumentGoNames }

// RequiredArguments returns the upstream mapstructure paths set with
// OverrideArgumentRequired.
func (c Component) RequiredArguments() map[string]bool { return c.requiredArguments }

// DefaultArguments returns the function set with OverrideDefaultArguments and
// the config type it expects, or nil when none is set.
func (c Component) DefaultArguments() (func(any), reflect.Type) {
	return c.defaultArguments, c.defaultArgumentsType
}

// DefaultArgumentsReason returns the reason given to OverrideDefaultArguments.
func (c Component) DefaultArgumentsReason() string { return c.defaultArgumentsReason }
