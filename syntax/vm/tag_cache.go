package vm

import (
	"reflect"
	"strings"
	"sync"

	"github.com/grafana/river/internal/rivertags"
)

// tagsCache caches the river tags for a struct type. This is never cleared,
// but since most structs will be statically created throughout the lifetime
// of the process, this will consume a negligible amount of memory.
var tagsCache sync.Map

func getCachedTagInfo(t reflect.Type) *tagInfo {
	if t.Kind() != reflect.Struct {
		panic("getCachedTagInfo called with non-struct type")
	}

	if entry, ok := tagsCache.Load(t); ok {
		return entry.(*tagInfo)
	}

	tfs := rivertags.Get(t)
	ti := &tagInfo{
		Tags:       tfs,
		TagLookup:  make(map[string]rivertags.Field, len(tfs)),
		EnumLookup: make(map[string]enumBlock), // The length is not known ahead of time
	}

	for _, tf := range tfs {
		switch {
		case tf.IsAttr(), tf.IsBlock():
			fullName := strings.Join(tf.Name, ".")
			ti.TagLookup[fullName] = tf

		case tf.IsEnum():
			fullName := strings.Join(tf.Name, ".")

			// Find all the blocks that match to the enum, and inject them into the
			// EnumLookup table.
			enumFieldType := t.FieldByIndex(tf.Index).Type
			enumBlocksInfo := getCachedTagInfo(deferenceType(enumFieldType.Elem()))
			for _, blockField := range enumBlocksInfo.TagLookup {
				// The full name of the enum block is the name of the enum plus the
				// name of the block, separated by '.'
				enumBlockName := fullName + "." + strings.Join(blockField.Name, ".")
				ti.EnumLookup[enumBlockName] = enumBlock{
					EnumField:  tf,
					BlockField: blockField,
				}
			}
		}
	}

	tagsCache.Store(t, ti)
	return ti
}

func deferenceType(ty reflect.Type) reflect.Type {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	return ty
}

type tagInfo struct {
	Tags      []rivertags.Field
	TagLookup map[string]rivertags.Field

	// EnumLookup maps enum blocks to the enum field. For example, an enum block
	// called "foo.foo" and "foo.bar" will both map to the "foo" enum field.
	EnumLookup map[string]enumBlock
}

type enumBlock struct {
	EnumField  rivertags.Field // Field in the parent struct of the enum slice
	BlockField rivertags.Field // Field in the enum struct for the enum block
}
