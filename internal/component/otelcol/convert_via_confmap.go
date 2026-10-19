package otelcol

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// ConvertOption customizes ConvertViaConfmap. Paths are dotted Alloy names
// (for example "policy.latency"); `,squash` fields add no segment.
type ConvertOption func(*convertConfig)

type convertConfig struct {
	skip    map[string]bool
	renames map[string]convertRename

	// elemDepth counts the repeated blocks being encoded around the current field.
	elemDepth int
}

type convertRename struct {
	upstream string
	convert  func(encoded any) (any, error)
}

// WithConvertSkip excludes Alloy-only fields (and everything under them).
func WithConvertSkip(paths ...string) ConvertOption {
	return func(c *convertConfig) {
		for _, p := range paths {
			c.skip[p] = true
		}
	}
}

// WithConvertRename emits the field at path under upstreamName. convert,
// when non-nil, receives the already-encoded value (int64, string, ...).
func WithConvertRename(path, upstreamName string, convert func(encoded any) (any, error)) ConvertOption {
	return func(c *convertConfig) {
		c.renames[path] = convertRename{upstream: upstreamName, convert: convert}
	}
}

// ConvertViaConfmap converts Alloy args into an upstream config by emitting a
// string map keyed by upstream mapstructure names and decoding it onto
// factory.CreateDefaultConfig() with confmap, which is how the collector
// loads its own config (including custom Unmarshal methods).
//
// Scalars are always emitted. Pointer fields (optional blocks) are emitted only
// when non-nil, since that is the signal that the user wrote the block. Inside
// repeated blocks, optional blocks left at their default aren't emitted either.
func ConvertViaConfmap(args any, factory interface{ CreateDefaultConfig() component.Config }, opts ...ConvertOption) (component.Config, error) {
	cfg := &convertConfig{
		skip:    map[string]bool{},
		renames: map[string]convertRename{},
	}
	for _, o := range opts {
		o(cfg)
	}

	rv := reflect.ValueOf(args)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("args is nil")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("args must be a struct, got %T", args)
	}

	m := map[string]any{}
	if err := cfg.emitStruct(rv, "", m); err != nil {
		return nil, err
	}

	out := factory.CreateDefaultConfig()
	if err := confmap.NewFromStringMap(m).Unmarshal(out); err != nil {
		return nil, err
	}
	return out, nil
}

// newDefault returns a zero value of t with SetToDefault applied when t has one.
func newDefault(t reflect.Type) reflect.Value {
	p := reflect.New(t)
	if d, ok := p.Interface().(interface{ SetToDefault() }); ok {
		d.SetToDefault()
	}
	return p.Elem()
}

// emitStruct writes the fields of v into dst.
func (c *convertConfig) emitStruct(v reflect.Value, path string, dst map[string]any) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, ok := sf.Tag.Lookup("alloy")
		if !sf.IsExported() || !ok {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		squash := false
		optionalBlock := slices.Contains(parts[1:], "block") && slices.Contains(parts[1:], "optional")
		for _, p := range parts[1:] {
			switch p {
			case "squash":
				squash = true
			case "label", "enum":
				name = ""
			}
		}
		if squash {
			fv := v.Field(i)
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					break
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				if err := c.emitStruct(fv, path, dst); err != nil {
					return err
				}
			}
			continue
		}
		if name == "" {
			continue
		}

		fieldPath := joinPath(path, name)
		if c.skip[fieldPath] {
			continue
		}
		fv := v.Field(i)
		for fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				break
			}
			fv = fv.Elem()
		}
		if fv.Kind() == reflect.Pointer || ((fv.Kind() == reflect.Slice || fv.Kind() == reflect.Map) && fv.IsNil()) {
			continue
		}
		// Upstream list elements decode onto zero values, so an unwritten block
		// adds nothing, and its zero values don't always decode (an empty
		// ottl.ErrorMode).
		if c.elemDepth > 0 && optionalBlock && fv.Kind() == reflect.Struct && reflect.DeepEqual(fv.Interface(), newDefault(fv.Type()).Interface()) {
			continue
		}

		enc, err := c.encode(fv, fieldPath)
		if err != nil {
			return fmt.Errorf("%s: %w", fieldPath, err)
		}

		key := name
		if r, ok := c.renames[fieldPath]; ok {
			key = r.upstream
			if r.convert != nil {
				if enc, err = r.convert(enc); err != nil {
					return fmt.Errorf("%s: %w", fieldPath, err)
				}
			}
		}
		dst[key] = enc
	}
	return nil
}

var durationType = reflect.TypeFor[time.Duration]()

// encode turns an Alloy value into something confmap's decode hooks accept.
func (c *convertConfig) encode(v reflect.Value, path string) (any, error) {
	if v.Type() == durationType {
		return time.Duration(v.Int()).String(), nil
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.String:
		return v.String(), nil // includes alloytypes.Secret
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return v.Float(), nil
	case reflect.Slice, reflect.Array:
		c.elemDepth++
		defer func() { c.elemDepth-- }()
		out := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			e, err := c.encode(v.Index(i), path)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("unsupported map key type %s", v.Type().Key())
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			e, err := c.encode(iter.Value(), path)
			if err != nil {
				return nil, err
			}
			out[iter.Key().String()] = e
		}
		return out, nil
	case reflect.Struct:
		out := map[string]any{}
		if err := c.emitStruct(v, path, out); err != nil {
			return nil, err
		}
		return out, nil
	}
	return nil, fmt.Errorf("no map encoding for %s", v.Type())
}
