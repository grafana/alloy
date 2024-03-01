package value

import (
	"reflect"

	"github.com/grafana/river/internal/reflectutil"
)

// structWrapper allows for partially traversing structs which contain fields
// representing blocks. This is required due to how block names and labels
// change the object representation.
//
// If a block name is a.b.c, then it is represented as three nested objects:
//
//	{
//	  a = {
//	    b = {
//	      c = { /* block contents */ },
//	    },
//	  }
//	}
//
// Similarly, if a block name is labeled (a.b.c "label"), then the label is the
// top-level key after c.
//
// structWrapper exposes Len, Keys, and Key methods similar to Value to allow
// traversing through the synthetic object. The values it returns are
// structWrappers.
//
// Code in value.go MUST check to see if a struct is a structWrapper *before*
// checking the value kind to ensure the appropriate methods are invoked.
type structWrapper struct {
	structVal reflect.Value
	fields    *objectFields
	label     string // Non-empty string if this struct is wrapped in a label.
}

func wrapStruct(val reflect.Value, keepLabel bool) structWrapper {
	if val.Kind() != reflect.Struct {
		panic("river/value: wrapStruct called on non-struct value")
	}

	fields := getCachedTags(val.Type())

	var label string
	if f, ok := fields.LabelField(); ok && keepLabel {
		label = reflectutil.Get(val, f).String()
	}

	return structWrapper{
		structVal: val,
		fields:    fields,
		label:     label,
	}
}

// Value turns sw into a value.
func (sw structWrapper) Value() Value {
	return Value{
		rv: reflect.ValueOf(sw),
		ty: TypeObject,
	}
}

func (sw structWrapper) Len() int {
	if len(sw.label) > 0 {
		return 1
	}
	return sw.fields.Len()
}

func (sw structWrapper) Keys() []string {
	if len(sw.label) > 0 {
		return []string{sw.label}
	}
	return sw.fields.Keys()
}

func (sw structWrapper) Key(key string) (index Value, ok bool) {
	if len(sw.label) > 0 {
		if key != sw.label {
			return
		}
		next := reflect.ValueOf(structWrapper{
			structVal: sw.structVal,
			fields:    sw.fields,
			// Unset the label now that we've traversed it
		})
		return Value{rv: next, ty: TypeObject}, true
	}

	keyType := sw.fields.Has(key)

	switch keyType {
	case objectKeyTypeInvalid:
		return // No such key

	case objectKeyTypeNestedField:
		// Continue traversing.
		nextNode, _ := sw.fields.NestedField(key)
		return Value{
			rv: reflect.ValueOf(structWrapper{
				structVal: sw.structVal,
				fields:    nextNode,
			}),
			ty: TypeObject,
		}, true

	case objectKeyTypeField:
		f, _ := sw.fields.Field(key)
		val, err := sw.structVal.FieldByIndexErr(f.Index)
		if err != nil {
			return Null, true
		}
		return makeValue(val), true
	}

	panic("river/value: unreachable")
}
