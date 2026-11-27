package bridge

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

// ProcessorArguments are the arguments of a bridged processor component. T is
// the upstream configuration struct, such as batchprocessor.Config.
//
// The fields of T are exposed as top-level attributes and blocks named after
// their `mapstructure` tags, next to the Alloy-only output and debug_metrics
// blocks.
type ProcessorArguments[T any] struct {
	Config T `alloy:",squash,mapstructure"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = ProcessorArguments[struct{}]{}
	_ syntax.Defaulter    = (*ProcessorArguments[struct{}])(nil)
	_ syntax.Unmarshaler  = (*ProcessorArguments[struct{}])(nil)
)

// processorArgumentsPlain has the same fields as ProcessorArguments but none
// of its methods, so it can be decoded without recursing into UnmarshalAlloy.
type processorArgumentsPlain[T any] ProcessorArguments[T]

func (args *processorArgumentsPlain[T]) SetToDefault() {
	(*ProcessorArguments[T])(args).SetToDefault()
}

// SetToDefault implements syntax.Defaulter.
func (args *ProcessorArguments[T]) SetToDefault() {
	*args = ProcessorArguments[T]{Config: *defaultConfig[T]()}
	args.DebugMetrics.SetToDefault()
}

// UnmarshalAlloy implements syntax.Unmarshaler.
//
// The body is evaluated twice. The first pass decodes into the typed
// arguments, so Alloy reports unknown names, blocks used as attributes, and
// values of the wrong type with source positions. The second pass decodes the
// same body into a BlockMap, which records exactly the settings the user
// wrote. Those settings are then decoded on top of the default configuration
// with confmap, so the result is what the Collector would produce from the
// equivalent YAML.
func (args *ProcessorArguments[T]) UnmarshalAlloy(f func(any) error) error {
	var typed processorArgumentsPlain[T]
	if err := f(&typed); err != nil {
		return err
	}

	var raw syntax.BlockMap
	if err := f(&raw); err != nil {
		return err
	}
	delete(raw, "output")
	delete(raw, "debug_metrics")

	cfg := defaultConfig[T]()
	if err := unmarshalConfig(raw, cfg); err != nil {
		return err
	}
	if err := validate(reflect.ValueOf(cfg)); err != nil {
		return err
	}

	typed.Config = *cfg
	*args = ProcessorArguments[T](typed)
	return nil
}

// Convert implements processor.Arguments.
func (args ProcessorArguments[T]) Convert() (otelcomponent.Config, error) {
	cfg := args.Config
	return &cfg, nil
}

// Extensions implements processor.Arguments.
func (args ProcessorArguments[T]) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements processor.Arguments.
func (args ProcessorArguments[T]) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args ProcessorArguments[T]) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args ProcessorArguments[T]) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// ProcessorOptions customizes how an upstream processor is exposed in Alloy.
type ProcessorOptions[T any] struct {
	// Name of the Alloy component. Defaults to "otelcol.processor.<type>",
	// where <type> is the upstream factory type.
	Name string

	// Stability of the Alloy component. Required.
	Stability featuregate.Stability

	// Defaults optionally overrides upstream defaults. Use it only to keep
	// backwards compatibility with defaults Alloy already documents.
	Defaults func(cfg *T)
}

// Processor describes a bridged processor.
type Processor struct {
	// Name of the Alloy component.
	Name string

	// Factory is the upstream processor factory.
	Factory otelprocessor.Factory

	// NewArguments returns the Alloy arguments for an upstream configuration.
	// cfg must have been created by Factory.
	NewArguments func(cfg otelcomponent.Config, output *otelcol.ConsumerArguments) component.Arguments
}

var processors sync.Map // map[string]Processor

// RegisterProcessor registers an upstream processor as an Alloy component.
// The Alloy schema, defaults and validation all come from the upstream
// configuration type T.
func RegisterProcessor[T any](factory otelprocessor.Factory, opts ProcessorOptions[T]) {
	name := opts.Name
	if name == "" {
		name = "otelcol.processor." + factory.Type().String()
	}

	register(factory, opts.Defaults)

	component.Register(component.Registration{
		Name:      name,
		Stability: opts.Stability,
		Args:      ProcessorArguments[T]{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(o component.Options, args component.Arguments) (component.Component, error) {
			return processor.New(o, factory, args.(ProcessorArguments[T]))
		},
	})

	processors.Store(name, Processor{
		Name:    name,
		Factory: factory,
		NewArguments: func(cfg otelcomponent.Config, output *otelcol.ConsumerArguments) component.Arguments {
			var args ProcessorArguments[T]
			args.SetToDefault()
			args.Config = *cfg.(*T)
			args.Output = output
			return &args
		},
	})
}

// Processors returns every bridged processor, sorted by name.
func Processors() []Processor {
	var out []Processor
	processors.Range(func(_, v any) bool {
		out = append(out, v.(Processor))
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// validate calls Validate on v and on every value nested inside it which
// implements it, in the same way the Collector validates a configuration
// before starting.
func validate(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}

	// Work on an addressable value so methods with pointer receivers apply.
	if v.Kind() != reflect.Pointer && !v.CanAddr() {
		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		v = cp
	}

	var errs []error

	target := v
	if v.Kind() != reflect.Pointer {
		target = v.Addr()
	}
	if !target.IsNil() {
		if validator, ok := target.Interface().(interface{ Validate() error }); ok {
			errs = append(errs, validator.Validate())
		}
	}

	elem := v
	for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Interface {
		if elem.IsNil() {
			return errors.Join(errs...)
		}
		elem = elem.Elem()
	}

	switch elem.Kind() {
	case reflect.Struct:
		for field := range elem.Type().Fields() {
			if field.IsExported() {
				errs = append(errs, validate(elem.FieldByIndex(field.Index)))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range elem.Len() {
			errs = append(errs, validate(elem.Index(i)))
		}
	case reflect.Map:
		iter := elem.MapRange()
		for iter.Next() {
			if err := validate(iter.Value()); err != nil {
				errs = append(errs, fmt.Errorf("%v: %w", iter.Key(), err))
			}
		}
	}
	return errors.Join(errs...)
}
