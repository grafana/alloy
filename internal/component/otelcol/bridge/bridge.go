// Package bridge exposes OpenTelemetry Collector components as Alloy
// components without hand-written argument structs.
//
// The Alloy schema of a bridged component is derived from the `mapstructure`
// tags of the upstream configuration struct. Values are decoded through
// confmap, the same way the Collector loads YAML, so upstream defaults, decode
// hooks, custom unmarshalers and validation all apply unchanged.
package bridge

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// registration is the information the bridge keeps about each upstream
// configuration type.
type registration struct {
	factory  otelcomponent.Factory
	defaults func(cfg any)
}

// registry maps an upstream configuration type (not a pointer) to its
// registration.
var registry sync.Map // map[reflect.Type]registration

func register[T any](factory otelcomponent.Factory, defaults func(*T)) {
	ty := reflect.TypeFor[T]()
	if _, ok := factory.CreateDefaultConfig().(*T); !ok {
		panic(fmt.Sprintf("bridge: factory %q creates %T, not *%s", factory.Type(), factory.CreateDefaultConfig(), ty))
	}

	reg := registration{factory: factory}
	if defaults != nil {
		reg.defaults = func(cfg any) { defaults(cfg.(*T)) }
	}
	if _, loaded := registry.LoadOrStore(ty, reg); loaded {
		panic(fmt.Sprintf("bridge: configuration type %s registered more than once", ty))
	}
}

// defaultConfig returns the default configuration for T: the upstream
// factory's defaults, followed by any Alloy-specific overrides.
func defaultConfig[T any]() *T {
	v, ok := registry.Load(reflect.TypeFor[T]())
	if !ok {
		panic(fmt.Sprintf("bridge: configuration type %s is not registered", reflect.TypeFor[T]()))
	}
	reg := v.(registration)

	cfg := reg.factory.CreateDefaultConfig().(*T)
	if reg.defaults != nil {
		reg.defaults(cfg)
	}
	return cfg
}

// unmarshalConfig decodes raw, the settings a user wrote in Alloy syntax, on
// top of cfg using confmap.
func unmarshalConfig[T any](raw map[string]any, cfg *T) error {
	normalized, err := normalize(raw, reflect.TypeFor[T]())
	if err != nil {
		return err
	}
	return confmap.NewFromStringMap(normalized.(map[string]any)).Unmarshal(cfg)
}

// normalize reshapes a value decoded from Alloy syntax into the shape confmap
// expects for a field of type ty:
//
//   - Alloy can't tell a single block from a list with one block, so a map is
//     wrapped in a list when ty is a slice or array.
//   - Values with a string kind but a named type, such as alloytypes.Secret,
//     become plain strings so confmap's decode hooks recognize them.
func normalize(v any, ty reflect.Type) (any, error) {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}

	switch v := v.(type) {
	case map[string]any:
		switch ty.Kind() {
		case reflect.Struct:
			out := make(map[string]any, len(v))
			for key, val := range v {
				fieldType, ok := mapstructureField(ty, key)
				if !ok {
					// Leave unknown keys for confmap to report.
					out[key] = val
					continue
				}
				nv, err := normalize(val, fieldType)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				out[key] = nv
			}
			return out, nil

		case reflect.Map:
			out := make(map[string]any, len(v))
			for key, val := range v {
				nv, err := normalize(val, ty.Elem())
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				out[key] = nv
			}
			return out, nil

		case reflect.Slice, reflect.Array:
			nv, err := normalize(v, ty.Elem())
			if err != nil {
				return nil, err
			}
			return []any{nv}, nil
		}
		return v, nil

	case []any:
		if ty.Kind() != reflect.Slice && ty.Kind() != reflect.Array {
			return v, nil
		}
		out := make([]any, len(v))
		for i, val := range v {
			nv, err := normalize(val, ty.Elem())
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = nv
		}
		return out, nil
	}

	if rv := reflect.ValueOf(v); rv.IsValid() && rv.Kind() == reflect.String && rv.Type() != reflect.TypeFor[string]() {
		return rv.String(), nil
	}
	return v, nil
}

// mapstructureField returns the type of the field of struct type ty which
// confmap decodes key into, following squashed fields.
func mapstructureField(ty reflect.Type, key string) (reflect.Type, bool) {
	for field := range ty.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if name == "-" {
			continue
		}
		if slices.Contains(strings.Split(opts, ","), "squash") {
			inner := field.Type
			for inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			if inner.Kind() == reflect.Struct {
				if t, ok := mapstructureField(inner, key); ok {
					return t, true
				}
			}
			continue
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		if name == key {
			return field.Type, true
		}
	}
	return nil, false
}
