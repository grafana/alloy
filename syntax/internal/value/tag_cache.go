package value

import (
	"reflect"

	"github.com/grafana/river/internal/rivertags"
)

// tagsCache caches the river tags for a struct type. This is never cleared,
// but since most structs will be statically created throughout the lifetime
// of the process, this will consume a negligible amount of memory.
var tagsCache = make(map[reflect.Type]*objectFields)

func getCachedTags(t reflect.Type) *objectFields {
	if t.Kind() != reflect.Struct {
		panic("getCachedTags called with non-struct type")
	}

	if entry, ok := tagsCache[t]; ok {
		return entry
	}

	ff := rivertags.Get(t)

	// Build a tree of keys.
	tree := &objectFields{
		fields:       make(map[string]rivertags.Field),
		nestedFields: make(map[string]*objectFields),
		keys:         []string{},
	}

	for _, f := range ff {
		if f.Flags&rivertags.FlagLabel != 0 {
			// Skip over label tags.
			tree.labelField = f
			continue
		}

		node := tree
		for i, name := range f.Name {
			// Add to the list of keys if this is a new key.
			if node.Has(name) == objectKeyTypeInvalid {
				node.keys = append(node.keys, name)
			}

			if i+1 == len(f.Name) {
				// Last fragment, add as a field.
				node.fields[name] = f
				continue
			}

			inner, ok := node.nestedFields[name]
			if !ok {
				inner = &objectFields{
					fields:       make(map[string]rivertags.Field),
					nestedFields: make(map[string]*objectFields),
					keys:         []string{},
				}
				node.nestedFields[name] = inner
			}
			node = inner
		}
	}

	tagsCache[t] = tree
	return tree
}

// objectFields is a parsed tree of fields in rivertags. It forms a tree where
// leaves are nested fields (e.g., for block names that have multiple name
// fragments) and nodes are the fields themselves.
type objectFields struct {
	fields       map[string]rivertags.Field
	nestedFields map[string]*objectFields
	keys         []string // Combination of fields + nestedFields
	labelField   rivertags.Field
}

type objectKeyType int

const (
	objectKeyTypeInvalid objectKeyType = iota
	objectKeyTypeField
	objectKeyTypeNestedField
)

// Has returns whether name exists as a field or a nested key inside keys.
// Returns objectKeyTypeInvalid if name does not exist as either.
func (of *objectFields) Has(name string) objectKeyType {
	if _, ok := of.fields[name]; ok {
		return objectKeyTypeField
	}
	if _, ok := of.nestedFields[name]; ok {
		return objectKeyTypeNestedField
	}
	return objectKeyTypeInvalid
}

// Len returns the number of named keys.
func (of *objectFields) Len() int { return len(of.keys) }

// Keys returns all named keys (fields and nested fields).
func (of *objectFields) Keys() []string { return of.keys }

// Field gets a non-nested field. Returns false if name is a nested field.
func (of *objectFields) Field(name string) (rivertags.Field, bool) {
	f, ok := of.fields[name]
	return f, ok
}

// NestedField gets a named nested field entry. Returns false if name is not a
// nested field.
func (of *objectFields) NestedField(name string) (*objectFields, bool) {
	nk, ok := of.nestedFields[name]
	return nk, ok
}

// LabelField returns the field used for the label (if any).
func (of *objectFields) LabelField() (rivertags.Field, bool) {
	return of.labelField, of.labelField.Index != nil
}
