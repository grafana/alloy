package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type fooArgs struct {
	Something bool `river:"something,attr"`
}
type fooExports struct {
	SomethingElse bool `river:"something_else,attr"`
}

type barArgs struct {
	Number int `river:"number,attr"`
}

func TestValueCache(t *testing.T) {
	vc := newValueCache()

	// Emulate values from the following River file:
	//
	//     foo {
	//       something = true
	//
	//       // Exported fields:
	//       // something_else = true
	//     }
	//
	//     bar "label_a" {
	//       number = 12
	//     }
	//
	//     bar "label_b" {
	//       number = 34
	//     }
	//
	// and expects to generate the equivalent to the following River object:
	//
	//     {
	//      	foo = {
	//      		something_else = true,
	//      	},
	//
	//      	bar = {
	//      		label_a = {},
	//      		label_b = {},
	//      	}
	//     }
	//
	// For now, only exports are placed in generated objects, which is why the
	// bar values are empty and the foo object only contains the exports.

	vc.CacheArguments(ComponentID{"foo"}, fooArgs{Something: true})
	vc.CacheExports(ComponentID{"foo"}, fooExports{SomethingElse: true})
	vc.CacheArguments(ComponentID{"bar", "label_a"}, barArgs{Number: 12})
	vc.CacheArguments(ComponentID{"bar", "label_b"}, barArgs{Number: 34})

	res := vc.BuildContext()

	var (
		expectKeys = []string{"foo", "bar"}
		actualKeys []string
	)
	for varName := range res.Variables {
		actualKeys = append(actualKeys, varName)
	}
	require.ElementsMatch(t, expectKeys, actualKeys)

	type object = map[string]interface{}

	expectFoo := fooExports{SomethingElse: true}
	expectBar := object{
		"label_a": object{}, // no exports for bar
		"label_b": object{}, // no exports for bar
	}
	require.Equal(t, expectFoo, res.Variables["foo"])
	require.Equal(t, expectBar, res.Variables["bar"])
}

func TestExportValueCache(t *testing.T) {
	vc := newValueCache()
	vc.CacheModuleExportValue("t1", 1)
	index := 0
	require.True(t, vc.ExportChangeIndex() != index)
	index = vc.ExportChangeIndex()
	require.False(t, vc.ExportChangeIndex() != index)

	vc.CacheModuleExportValue("t1", 2)
	require.True(t, vc.ExportChangeIndex() != index)
	index = vc.ExportChangeIndex()
	require.False(t, vc.ExportChangeIndex() != index)

	vc.CacheModuleExportValue("t1", 2)
	require.False(t, vc.ExportChangeIndex() != index)

	index = vc.ExportChangeIndex()
	vc.CacheModuleExportValue("t2", "test")
	require.True(t, vc.ExportChangeIndex() != index)

	index = vc.ExportChangeIndex()
	vc.ClearModuleExports()
	require.True(t, vc.ExportChangeIndex() != index)
}

func TestExportValueCacheUncomparable(t *testing.T) {
	vc := newValueCache()
	type test struct {
		TM map[string]string
	}
	// This test for an uncomparable error that is triggered when you do a simple `v == v2` comparison,
	// instead using deep equals does a smarter approach.
	vc.CacheModuleExportValue("t2", test{TM: map[string]string{}})
	index := vc.ExportChangeIndex()
	vc.CacheModuleExportValue("t2", test{TM: map[string]string{}})
	require.Equal(t, index, vc.moduleChangedIndex)
}

func TestModuleArgumentCache(t *testing.T) {
	tt := []struct {
		name     string
		argValue any
	}{
		{
			name:     "Nil",
			argValue: nil,
		},
		{
			name:     "Number",
			argValue: 1,
		},
		{
			name:     "String",
			argValue: "string",
		},
		{
			name:     "Bool",
			argValue: true,
		},
		{
			name:     "Map",
			argValue: map[string]any{"test": "map"},
		},
		{
			name:     "Capsule",
			argValue: fooExports{SomethingElse: true},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create and cache the argument
			vc := newValueCache()
			vc.CacheModuleArgument("arg", tc.argValue)

			// Build the scope and validate it
			res := vc.BuildContext()
			expected := map[string]any{"arg": map[string]any{"value": tc.argValue}}
			require.Equal(t, expected, res.Variables["argument"])

			// Sync arguments where the arg shouldn't change
			syncArgs := map[string]any{"arg": tc.argValue}
			vc.SyncModuleArgs(syncArgs)
			res = vc.BuildContext()
			require.Equal(t, expected, res.Variables["argument"])

			// Sync arguments where the arg should clear out
			syncArgs = map[string]any{}
			vc.SyncModuleArgs(syncArgs)
			res = vc.BuildContext()
			require.Equal(t, map[string]any{}, res.Variables)
		})
	}
}
