package otelcol

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// DefaultsOption customizes DefaultsFrom.
type DefaultsOption func(*defaultsConfig)

type defaultsConfig struct {
	// renames is keyed by the Alloy attribute/block name. Names match at any depth.
	renames map[string]rename
}

type rename struct {
	upstream string
	convert  func(upstream reflect.Value) (any, error)
}

// WithRename maps the upstream field upstreamName onto the Alloy field
// alloyName. convert, when non-nil, turns the upstream value into something
// assignable to the Alloy field (a unit conversion, for example).
func WithRename(upstreamName, alloyName string, convert func(upstream reflect.Value) (any, error)) DefaultsOption {
	return func(c *defaultsConfig) {
		c.renames[alloyName] = rename{upstream: upstreamName, convert: convert}
	}
}

// DefaultsFrom copies defaults from an upstream config (usually
// factory.CreateDefaultConfig()) into Alloy args. Fields are matched by name:
// the first part of the upstream `mapstructure` tag against the first part of
// the Alloy `alloy` tag, with `,squash` honored on both sides.
//
// Alloy fields with no upstream counterpart are left untouched, so callers
// should set Alloy-only fields first. Only scalar leaves are copied: Alloy
// pointer (optional block) fields are never allocated, and an upstream
// configoptional.Optional without a value (None or Default) leaves the Alloy
// field as is.
func DefaultsFrom(upstream any, args any, opts ...DefaultsOption) error {
	cfg := &defaultsConfig{renames: map[string]rename{}}
	for _, o := range opts {
		o(cfg)
	}

	up := reflect.ValueOf(upstream)
	for up.IsValid() && (up.Kind() == reflect.Pointer || up.Kind() == reflect.Interface) {
		if up.IsNil() {
			return fmt.Errorf("upstream config is nil")
		}
		up = up.Elem()
	}
	al := reflect.ValueOf(args)
	if al.Kind() != reflect.Pointer || al.IsNil() || al.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("args must be a non-nil pointer to a struct, got %T", args)
	}
	if up.Kind() != reflect.Struct {
		return fmt.Errorf("upstream must be a struct or pointer to one, got %T", upstream)
	}
	return copyStruct(cfg, up, al.Elem(), "")
}

func copyStruct(cfg *defaultsConfig, up, al reflect.Value, path string) error {
	upFields := map[string]reflect.Value{}
	collectFields(up, "mapstructure", upFields)
	alFields := map[string]reflect.Value{}
	collectFields(al, "alloy", alFields)

	for name, alField := range alFields {
		fieldPath := joinPath(path, name)
		var upField reflect.Value
		var conv func(reflect.Value) (any, error)
		if r, ok := cfg.renames[name]; ok {
			f, ok := upFields[r.upstream]
			if !ok {
				return fmt.Errorf("%s: renamed upstream field %q not found", fieldPath, r.upstream)
			}
			upField, conv = f, r.convert
		} else if f, ok := upFields[name]; ok {
			upField = f
		} else {
			continue
		}

		if conv != nil {
			v, err := conv(upField)
			if err != nil {
				return fmt.Errorf("%s: %w", fieldPath, err)
			}
			upField = reflect.ValueOf(v)
		}
		if err := assign(cfg, upField, alField, fieldPath); err != nil {
			return err
		}
	}
	return nil
}

// collectFields indexes the exported fields of struct v by the first part of
// the given tag, flattening `,squash` fields. A mapstructure field with no
// name uses the Go field name; "-" is skipped. Alloy fields without a tag are
// skipped.
func collectFields(v reflect.Value, tagKey string, out map[string]reflect.Value) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag, hasTag := sf.Tag.Lookup(tagKey)
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "-" || (tagKey == "alloy" && !hasTag) {
			continue
		}
		if slices.Contains(parts[1:], "squash") {
			fv := v.Field(i)
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					break
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				collectFields(fv, tagKey, out)
			}
			continue
		}
		if name == "" {
			name = sf.Name
		}
		out[name] = v.Field(i)
	}
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func assign(cfg *defaultsConfig, up, al reflect.Value, path string) error {
	if !up.IsValid() {
		return nil
	}

	// Unwrap upstream pointers and Optionals.
	for {
		if up.Kind() == reflect.Pointer || up.Kind() == reflect.Interface {
			if up.IsNil() {
				return nil
			}
			up = up.Elem()
			continue
		}
		if inner, isOptional, hasValue := unwrapOptional(up); isOptional {
			if !hasValue {
				return nil
			}
			up = inner
			continue
		}
		break
	}

	// Pointer fields are optional blocks. Allocating one would change what an
	// omitted block means, so only scalar leaves are ever copied.
	if al.Kind() == reflect.Pointer {
		return nil
	}

	switch {
	case up.Kind() == reflect.Struct && al.Kind() == reflect.Struct && hasAlloyTags(al.Type()):
		return copyStruct(cfg, up, al, path)

	case up.Type().AssignableTo(al.Type()):
		al.Set(copyValue(up))
		return nil

	case isIntKind(up.Kind()) && isIntKind(al.Kind()):
		return assignInt(up, al, path)

	case up.Kind() == al.Kind() && up.Type().ConvertibleTo(al.Type()) && isScalarKind(up.Kind()):
		al.Set(up.Convert(al.Type()))
		return nil

	case up.Kind() == reflect.Slice && al.Kind() == reflect.Slice && up.Type().Elem().Kind() == al.Type().Elem().Kind() && isScalarKind(up.Type().Elem().Kind()) && up.Type().Elem().ConvertibleTo(al.Type().Elem()):
		s := reflect.MakeSlice(al.Type(), up.Len(), up.Len())
		for i := 0; i < up.Len(); i++ {
			s.Index(i).Set(up.Index(i).Convert(al.Type().Elem()))
		}
		al.Set(s)
		return nil

	// An unset upstream list with no scalar mapping (such as repeated blocks)
	// has nothing to copy.
	case (up.Kind() == reflect.Slice || up.Kind() == reflect.Map) && up.IsNil():
		return nil
	}

	return fmt.Errorf("%s: cannot map upstream %s to Alloy %s", path, up.Type(), al.Type())
}

func isScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func hasAlloyTags(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if _, ok := t.Field(i).Tag.Lookup("alloy"); ok {
			return true
		}
	}
	return false
}

// copyValue copies slices and maps so Alloy args never alias the upstream
// default config.
func copyValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		s := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(s, v)
		return s
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		m := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, k := range v.MapKeys() {
			m.SetMapIndex(k, v.MapIndex(k))
		}
		return m
	}
	return v
}

// unwrapOptional detects configoptional.Optional[T] and returns the T value
// when it has one. Only the Some flavor has a value (HasValue).
func unwrapOptional(v reflect.Value) (inner reflect.Value, isOptional, hasValue bool) {
	t := v.Type()
	if t.Kind() != reflect.Struct || t.PkgPath() != "go.opentelemetry.io/collector/config/configoptional" || !strings.HasPrefix(t.Name(), "Optional[") {
		return reflect.Value{}, false, false
	}
	p := reflect.New(t)
	p.Elem().Set(v)
	if !p.MethodByName("HasValue").Call(nil)[0].Bool() {
		return reflect.Value{}, true, false
	}
	return p.MethodByName("Get").Call(nil)[0], true, true
}

func isIntKind(k reflect.Kind) bool {
	return (k >= reflect.Int && k <= reflect.Int64) || (k >= reflect.Uint && k <= reflect.Uint64)
}

// assignInt converts between integer kinds, erroring instead of truncating.
func assignInt(up, al reflect.Value, path string) error {
	overflow := fmt.Errorf("%s: value %v overflows Alloy %s", path, up.Interface(), al.Type())
	switch {
	case up.CanInt() && al.CanInt():
		if al.OverflowInt(up.Int()) {
			return overflow
		}
		al.SetInt(up.Int())
	case up.CanInt():
		if up.Int() < 0 || al.OverflowUint(uint64(up.Int())) {
			return overflow
		}
		al.SetUint(uint64(up.Int()))
	case al.CanInt():
		if up.Uint() > 1<<63-1 || al.OverflowInt(int64(up.Uint())) {
			return overflow
		}
		al.SetInt(int64(up.Uint()))
	default:
		if al.OverflowUint(up.Uint()) {
			return overflow
		}
		al.SetUint(up.Uint())
	}
	return nil
}
