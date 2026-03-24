package tagcache

import (
	"reflect"
	"strings"
	"sync"

	"github.com/grafana/alloy/syntax/internal/syntaxtags"
)

// tagsCache caches the alloy tags for a struct type. This is never cleared,
// but since most structs will be statically created throughout the lifetime
// of the process, this will consume a negligible amount of memory.
var tagsCache sync.Map

func Get(t reflect.Type) *TagInfo {
	if t.Kind() != reflect.Struct {
		panic("getCachedTagInfo called with non-struct type")
	}

	if entry, ok := tagsCache.Load(t); ok {
		return entry.(*TagInfo)
	}

	tfs := syntaxtags.Get(t)
	ti := &TagInfo{
		Tags:       tfs,
		TagLookup:  make(map[string]syntaxtags.Field, len(tfs)),
		EnumLookup: make(map[string]EnumBlock), // The length is not known ahead of time
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
			enumBlocksInfo := Get(deferenceType(enumFieldType.Elem()))
			for _, blockField := range enumBlocksInfo.TagLookup {
				// The full name of the enum block is the name of the enum plus the
				// name of the block, separated by '.'
				enumBlockName := fullName + "." + strings.Join(blockField.Name, ".")
				ti.EnumLookup[enumBlockName] = EnumBlock{
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

type TagInfo struct {
	Tags      []syntaxtags.Field
	TagLookup map[string]syntaxtags.Field

	// EnumLookup maps enum blocks to the enum field. For example, an enum block
	// called "foo.foo" and "foo.bar" will both map to the "foo" enum field.
	EnumLookup map[string]EnumBlock
}

type EnumBlock struct {
	EnumField  syntaxtags.Field // Field in the parent struct of the enum slice
	BlockField syntaxtags.Field // Field in the enum struct for the enum block
}
