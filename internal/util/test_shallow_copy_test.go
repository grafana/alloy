package util

import (
	"maps"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/strings/slices"
)

type copyable interface {
	Copy() any
}

// String pointer
type withStringPointer struct {
	a *string
}

type withStringPointer_Shallow withStringPointer
type withStringPointer_Deep withStringPointer

func (s withStringPointer_Shallow) Copy() any {
	return s
}

func (s withStringPointer_Deep) Copy() any {
	strCopy := *s.a
	return withStringPointer_Deep{
		a: &strCopy,
	}
}

// Map
type withMap struct {
	a map[string]string
}

type withMap_Shallow withMap
type withMap_Deep withMap

func (s withMap_Shallow) Copy() any {
	return s
}

func (s withMap_Deep) Copy() any {
	return withMap_Deep{
		a: maps.Clone(s.a),
	}
}

// Slice
type withSlice struct {
	a []string
}

type withSlice_Shallow withSlice
type withSlice_Deep withSlice

func (s withSlice_Shallow) Copy() any {
	return s
}

func (s withSlice_Deep) Copy() any {
	return withSlice_Deep{
		a: slices.Clone(s.a),
	}
}

// Primitive types
type withPrimitiveTypes struct {
	a int
	b bool
}

func (s withPrimitiveTypes) Copy() any {
	return s
}

// Utility functions
func newStrPtr(s string) *string {
	return &s
}

// Test for shallow and deep copying
func Test_ShallowCopy(t *testing.T) {
	tests := map[string]struct {
		input             copyable
		expectShallowCopy bool
	}{
		"stringPtrShallow": {
			input: withStringPointer_Shallow{
				a: newStrPtr("test"),
			},
			expectShallowCopy: true,
		},
		"stringPtrDeep": {
			input: withStringPointer_Deep{
				a: newStrPtr("test"),
			},
			expectShallowCopy: false,
		},
		"mapShallow": {
			input: withMap_Shallow{
				a: map[string]string{"a": "b"},
			},
			expectShallowCopy: true,
		},
		"mapDeep": {
			input: withMap_Deep{
				a: map[string]string{"a": "b"},
			},
			expectShallowCopy: false,
		},
		"sliceShallow": {
			input: withSlice_Shallow{
				a: []string{"a", "b"},
			},
			expectShallowCopy: true,
		},
		"sliceDeep": {
			input: withSlice_Deep{
				a: []string{"a", "b"},
			},
			expectShallowCopy: false,
		},
		"primitiveTypes": {
			input: withPrimitiveTypes{
				a: 1,
				b: true,
			},
			expectShallowCopy: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			copiedInput := test.input.Copy()
			_, shared := SharePointer(reflect.ValueOf(test.input), reflect.ValueOf(copiedInput), false)
			if test.expectShallowCopy {
				require.True(t, shared, "expected a shallow copy")
			} else {
				require.False(t, shared, "expected a deep copy")
			}
		})
	}
}
