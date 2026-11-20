package otelgen

import (
	"reflect"

	"github.com/grafana/alloy/internal/featuregate"
)

// OverrideComponentName registers the component as otelcol.<kind>.<name>
// instead of using the upstream factory type.
func OverrideComponentName(name string) Option {
	return func(c *Component) { c.componentName = name }
}

// OverrideStability registers the component with stability instead of the one
// derived from the upstream signal stability. reason explains why Alloy differs
// from upstream and is written next to the generated registration.
func OverrideStability(reason string, stability featuregate.Stability) Option {
	return func(c *Component) {
		c.stability = stability
		c.stabilityReason = reason
	}
}

// OverrideArgumentName sets the Alloy argument name for the upstream field at
// path, its mapstructure names joined by dots (e.g. pass_through.gauge),
// instead of the mapstructure name.
func OverrideArgumentName(path, name string) Option {
	return func(c *Component) {
		if c.argumentNames == nil {
			c.argumentNames = map[string]string{}
		}
		c.argumentNames[path] = name
	}
}

// OverrideArgumentType replaces the type of the upstream field at path, its
// mapstructure names joined by dots, with T. The package must define the
// conversions by hand:
//
//	func <goName>ToUpstream(T) <upstream type>
//	func <goName>FromUpstream(<upstream type>) T
//
// with goName the field name in Arguments starting in lower case.
func OverrideArgumentType[T any](path string) Option {
	return func(c *Component) {
		if c.argumentTypes == nil {
			c.argumentTypes = map[string]reflect.Type{}
		}
		c.argumentTypes[path] = reflect.TypeFor[T]()
	}
}

// OverrideArgumentGoName sets the field name in Arguments for the upstream
// field at path, its mapstructure names joined by dots, instead of the
// upstream Go name.
func OverrideArgumentGoName(path, goName string) Option {
	return func(c *Component) {
		if c.argumentGoNames == nil {
			c.argumentGoNames = map[string]string{}
		}
		c.argumentGoNames[path] = goName
	}
}

// OverrideArgumentRequired makes the argument for the upstream field at path,
// its mapstructure names joined by dots, required instead of optional.
func OverrideArgumentRequired(path string) Option {
	return func(c *Component) {
		if c.requiredArguments == nil {
			c.requiredArguments = map[string]bool{}
		}
		c.requiredArguments[path] = true
	}
}

// OverrideDefaultArguments changes defaults, which fn receives, into Alloy's
// defaults. T is a pointer to either the factory's config type, for the
// defaults of Arguments, or the upstream type of a pointer, optional or
// repeated block, for the defaults of that block once it is set. Plain blocks
// take their defaults from the config. reason explains why Alloy differs from
// upstream and is written next to the generated defaults.
func OverrideDefaultArguments[T any](reason string, fn func(T)) Option {
	return func(c *Component) {
		c.defaultArguments = append(c.defaultArguments, DefaultArguments{
			Fn:     func(cfg any) { fn(cfg.(T)) },
			Type:   reflect.TypeFor[T](),
			Reason: reason,
		})
	}
}
