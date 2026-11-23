package syntaxtags

import (
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// mapstructureTypes records struct types which must be interpreted using
// `mapstructure` tags rather than `alloy` tags. Types are added when they are
// reached from a field tagged with `alloy:",squash,mapstructure"`.
var mapstructureTypes sync.Map // map[reflect.Type]struct{}

var goTextUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()

func isMapstructureType(ty reflect.Type) bool {
	_, ok := mapstructureTypes.Load(ty)
	return ok
}

// GetMapstructure returns the list of fields for a struct type ty, deriving
// them from `mapstructure` tags in the same way the OpenTelemetry Collector's
// confmap package does. GetMapstructure panics if ty is not a struct type.
//
// The rules are:
//
//   - The field name is the name in the `mapstructure` tag, or the Go field
//     name when the tag has no name. Fields tagged `mapstructure:"-"` and
//     unexported fields are skipped.
//   - Fields tagged `mapstructure:",squash"` expose their inner fields as
//     outer fields.
//   - Types implementing encoding.TextUnmarshaler are attributes.
//   - Structs and pointers to structs are blocks.
//   - Slices and arrays of structs are repeated blocks.
//   - Everything else is an attribute.
//   - Every field is optional. Required fields are enforced by the upstream
//     configuration's own validation.
//
// Every struct type used by a block is registered so that later calls to
// [Get] for that type also use these rules.
func GetMapstructure(ty reflect.Type) []Field {
	if k := ty.Kind(); k != reflect.Struct {
		panic(fmt.Sprintf("syntaxtags: GetMapstructure requires struct kind, got %s", k))
	}
	mapstructureTypes.Store(ty, struct{}{})

	var (
		fields    []Field
		usedNames = make(map[string][]int)
	)

	for field := range ty.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if name == "-" {
			continue
		}

		// Embedded structs may be squashed even when their type is unexported,
		// since their exported fields are still promoted.
		squash := slices.Contains(strings.Split(opts, ","), "squash")
		if !field.IsExported() && !(squash && field.Anonymous) {
			continue
		}

		if squash {
			innerType := deferenceType(field.Type)
			if !isStructType(innerType) {
				panic(fmt.Sprintf("syntaxtags: mapstructure squash field requires struct, got %s at %s", innerType, printPathToField(ty, field.Index)))
			}
			for _, inner := range GetMapstructure(innerType) {
				f := Field{
					Name:  inner.Name,
					Index: slices.Concat(field.Index, inner.Index),
					Flags: inner.Flags,
				}
				fields = appendUnique(fields, usedNames, ty, f)
			}
			continue
		}

		if name == "" {
			name = field.Name
		}
		if !isIdentifier(name) {
			panic(fmt.Sprintf("syntaxtags: mapstructure name %q at %s is not a valid identifier", name, printPathToField(ty, field.Index)))
		}

		f := Field{
			Name:  []string{name},
			Index: field.Index,
			Flags: FlagOptional,
		}
		if blockType, ok := mapstructureBlockType(field.Type); ok {
			f.Flags |= FlagBlock
			mapstructureTypes.Store(blockType, struct{}{})
		} else {
			f.Flags |= FlagAttr
			registerAttrStructs(field.Type)
		}
		fields = appendUnique(fields, usedNames, ty, f)
	}

	return fields
}

// mapstructureBlockType reports whether a field of type ty should be treated
// as a block, and returns the struct type the block decodes into.
func mapstructureBlockType(ty reflect.Type) (reflect.Type, bool) {
	if isTextUnmarshaler(ty) {
		return nil, false
	}
	base := deferenceType(ty)
	switch base.Kind() {
	case reflect.Struct:
		return base, true
	case reflect.Slice, reflect.Array:
		elem := base.Elem()
		if isTextUnmarshaler(elem) {
			return nil, false
		}
		if elem = deferenceType(elem); elem.Kind() == reflect.Struct {
			return elem, true
		}
	}
	return nil, false
}

// registerAttrStructs registers struct types nested inside an attribute's type
// (such as the values of a map[string]SomeStruct) so that they decode as
// objects instead of capsules.
func registerAttrStructs(ty reflect.Type) {
	for {
		if isTextUnmarshaler(ty) {
			return
		}
		switch ty.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			ty = ty.Elem()
		case reflect.Map:
			ty = ty.Elem()
		case reflect.Struct:
			if !isMapstructureType(ty) {
				mapstructureTypes.Store(ty, struct{}{})
			}
			return
		default:
			return
		}
	}
}

func isTextUnmarshaler(ty reflect.Type) bool {
	return ty.Implements(goTextUnmarshaler) || reflect.PointerTo(ty).Implements(goTextUnmarshaler)
}

func appendUnique(fields []Field, usedNames map[string][]int, ty reflect.Type, f Field) []Field {
	fullName := strings.Join(f.Name, ".")
	if first, used := usedNames[fullName]; used {
		panic(fmt.Sprintf("syntaxtags: field name %s already used by %s", fullName, printPathToField(ty, first)))
	}
	usedNames[fullName] = f.Index
	return append(fields, f)
}

// isIdentifier reports whether s is a valid Alloy identifier.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z':
		case i > 0 && '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return true
}
