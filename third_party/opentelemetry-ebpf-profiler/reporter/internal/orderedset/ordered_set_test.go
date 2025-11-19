package orderedset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedSet(t *testing.T) {
	for _, tt := range []struct {
		name string
		set  OrderedSet[string]
		key  string

		wantSet    OrderedSet[string]
		wantIndex  int32
		wantExists bool
	}{
		{
			name: "with a value not yet in the string map",
			set:  OrderedSet[string]{},
			key:  "foo",

			wantIndex:  0,
			wantSet:    OrderedSet[string]{"foo": 0},
			wantExists: false,
		},
		{
			name: "with a duplicate value already in the string map",
			set:  OrderedSet[string]{"foo": 0, "bar": 1},
			key:  "bar",

			wantIndex:  1,
			wantSet:    OrderedSet[string]{"foo": 0, "bar": 1},
			wantExists: true,
		},
		{
			name: "with a non-duplicate value already in the string map",
			set:  OrderedSet[string]{"foo": 0},
			key:  "baz",

			wantIndex:  1,
			wantSet:    OrderedSet[string]{"foo": 0, "baz": 1},
			wantExists: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			i, exists := tt.set.AddWithCheck(tt.key)
			assert.Equal(t, tt.wantIndex, i)
			assert.Equal(t, tt.wantSet, tt.set)
			assert.Equal(t, tt.wantExists, exists)
		})
	}
}
