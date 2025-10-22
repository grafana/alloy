package controller

import (
	"testing"

	"github.com/grafana/alloy/syntax/vm"
	"github.com/stretchr/testify/require"
)

type fooExports struct {
	SomethingElse bool `alloy:"something_else,attr"`
}

type barArgs struct {
	Number int `alloy:"number,attr"`
}

func TestValueCache(t *testing.T) {
	vc := newValueCache()

	// Emulate values from the following Alloy file:
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
	// and expects to generate the equivalent to the following Alloy object:
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

	require.NoError(t, vc.CacheExports(ComponentID{"foo"}, fooExports{SomethingElse: true}))

	res := vc.GetContext()

	var (
		expectKeys = []string{"foo"}
		actualKeys []string
	)
	for varName := range res.Variables {
		actualKeys = append(actualKeys, varName)
	}
	require.ElementsMatch(t, expectKeys, actualKeys)

	expectFoo := fooExports{SomethingElse: true}
	require.Equal(t, expectFoo, res.Variables["foo"])
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
			res := vc.GetContext()
			expected := map[string]any{"arg": map[string]any{"value": tc.argValue}}
			require.Equal(t, expected, res.Variables["argument"])

			// Sync arguments where the arg shouldn't change
			syncArgs := map[string]any{"arg": tc.argValue}
			vc.SyncModuleArgs(syncArgs)
			res = vc.GetContext()
			require.Equal(t, expected, res.Variables["argument"])

			// Sync arguments where the arg should clear out
			syncArgs = map[string]any{}
			vc.SyncModuleArgs(syncArgs)
			res = vc.GetContext()
			require.Equal(t, map[string]any{}, res.Variables)
		})
	}
}

func TestScope(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"scope": barArgs{Number: 13},
			},
		},
	)
	require.NoError(t, vc.CacheExports(ComponentID{"foo", "bar"}, barArgs{Number: 12}))
	res := vc.GetContext()

	expected := map[string]any{
		"test": map[string]any{
			"scope": barArgs{
				Number: 13,
			},
		},
		"foo": map[string]any{
			"bar": barArgs{
				Number: 12,
			},
		},
	}
	require.Equal(t, expected, res.Variables)
}

func TestScopeSameNamespace(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"scope": barArgs{Number: 13},
			},
		},
	)
	require.NoError(t, vc.CacheExports(ComponentID{"test", "bar"}, barArgs{Number: 12}))
	res := vc.GetContext()

	expected := map[string]any{
		"test": map[string]any{
			"scope": barArgs{
				Number: 13,
			},
			"bar": barArgs{
				Number: 12,
			},
		},
	}
	require.Equal(t, expected, res.Variables)
}

func TestScopeOverride(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"scope": barArgs{Number: 13},
			},
		},
	)
	require.NoError(t, vc.CacheExports(ComponentID{"test", "scope"}, barArgs{Number: 12}))
	res := vc.GetContext()

	expected := map[string]any{
		"test": map[string]any{
			"scope": barArgs{
				Number: 12,
			},
		},
	}
	require.Equal(t, expected, res.Variables)
}

func TestScopePathOverrideError(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": barArgs{Number: 13},
		},
	)
	require.ErrorContains(t,
		vc.CacheExports(ComponentID{"test", "scope"}, barArgs{Number: 12}),
		"expected a map but found a value for \"test\" when trying to cache the export for test.scope",
	)
}

func TestScopeComplex(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"cp1": map[string]any{
					"scope": barArgs{Number: 13},
				},
				"cp2": barArgs{Number: 12},
			},
		},
	)
	require.NoError(t, vc.CacheExports(ComponentID{"test", "cp1", "foo"}, barArgs{Number: 12}))
	require.NoError(t, vc.CacheExports(ComponentID{"test", "cp1", "bar", "fizz"}, barArgs{Number: 2}))
	res := vc.GetContext()

	expected := map[string]any{
		"test": map[string]any{
			"cp1": map[string]any{
				"scope": barArgs{Number: 13},
				"foo":   barArgs{Number: 12},
				"bar": map[string]any{
					"fizz": barArgs{Number: 2},
				},
			},
			"cp2": barArgs{Number: 12},
		},
	}
	require.Equal(t, expected, res.Variables)
}

func TestSyncIds(t *testing.T) {
	vc := newValueCache()
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"cp1": map[string]any{
					"scope": barArgs{Number: 13},
				},
				"cp2": barArgs{Number: 12},
			},
			"test2": map[string]any{
				"cp1": map[string]any{
					"scope": barArgs{Number: 13},
				},
			},
			"test3": 5,
		},
	)
	require.NoError(t, vc.CacheExports(ComponentID{"test", "cp1", "bar", "fizz"}, barArgs{Number: 2}))
	originalIds := map[string]ComponentID{
		"test.cp1":  {"test", "cp1"},
		"test.cp2":  {"test", "cp2"},
		"test2.cp1": {"test2", "cp1"},
		"test3":     {"test3"},
	}
	require.NoError(t, vc.SyncIDs(originalIds))
	require.Equal(t, originalIds, vc.componentIds)

	newIds := map[string]ComponentID{
		"test.cp1":  {"test", "cp1"},
		"test2.cp1": {"test2", "cp1"},
	}
	require.NoError(t, vc.SyncIDs(newIds))
	require.Equal(t, newIds, vc.componentIds)
	expected := map[string]any{
		"test": map[string]any{
			"cp1": map[string]any{
				"scope": barArgs{Number: 13},
				"bar": map[string]any{
					"fizz": barArgs{Number: 2},
				},
			},
		},
		"test2": map[string]any{
			"cp1": map[string]any{
				"scope": barArgs{Number: 13},
			},
		},
	}
	res := vc.GetContext()
	require.Equal(t, expected, res.Variables)
}

func TestSyncIdsError(t *testing.T) {
	vc := newValueCache()
	componentID := ComponentID{"test", "cp1", "bar", "fizz"}
	ids := map[string]ComponentID{"test.cp1.bar.fizz": componentID}
	require.NoError(t, vc.CacheExports(componentID, barArgs{Number: 2}))
	require.NoError(t, vc.SyncIDs(ids))
	require.Equal(t, ids, vc.componentIds)

	// Modify the map manually
	vc.componentIds = map[string]ComponentID{"test.cp1.bar.fizz.foo": {"test", "cp1", "bar", "fizz", "foo"}}
	require.ErrorContains(t,
		vc.SyncIDs(ids),
		"failed to sync component test.cp1.bar.fizz.foo: expected a map but found a value for \"fizz\"",
	)
}

func TestCapsule(t *testing.T) {
	vc := newValueCache()
	bar := barArgs{Number: 13}
	vc.scope = vm.NewScope(
		map[string]any{
			"test": map[string]any{
				"scope": &bar,
			},
		},
	)
	res := vc.GetContext()

	expected := map[string]any{
		"test": map[string]any{
			"scope": &bar,
		},
	}
	require.Equal(t, expected, res.Variables)
}
