package labelpack

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	commonlabels "github.com/prometheus/common/model"
	modellabels "github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collect returns all labels of l as a map, and asserts that Range yielded them
// in ascending name order.
func collect(t *testing.T, l Labels) map[string]string {
	t.Helper()
	out := map[string]string{}
	prev := ""
	first := true
	l.Range(func(name, value string) bool {
		if !first {
			assert.Less(t, prev, name, "labels must be yielded in ascending name order")
		}
		prev, first = name, false
		_, dup := out[name]
		assert.False(t, dup, "label %q yielded twice", name)
		out[name] = value
		return true
	})
	return out
}

func TestFromMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "empty",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "single",
			input:    map[string]string{"a": "1"},
			expected: map[string]string{"a": "1"},
		},
		{
			name:     "many, unsorted input",
			input:    map[string]string{"zed": "z", "abc": "a", "mno": "m", "b": "bb"},
			expected: map[string]string{"abc": "a", "b": "bb", "mno": "m", "zed": "z"},
		},
		{
			name:     "empty values dropped",
			input:    map[string]string{"a": "1", "b": "", "c": "3"},
			expected: map[string]string{"a": "1", "c": "3"},
		},
		{
			name:     "all values empty",
			input:    map[string]string{"a": "", "b": ""},
			expected: map[string]string{},
		},
		{
			// An empty name is not a valid label name, and Get can never report
			// one as present, so it is dropped rather than stored unreachably.
			name:     "empty name dropped",
			input:    map[string]string{"": "1", "a": "2"},
			expected: map[string]string{"a": "2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, n := FromMap(tc.input)
			require.Equal(t, len(tc.expected), n, "reported count")
			require.Equal(t, len(tc.expected), l.Len(), "Len()")
			require.Equal(t, tc.expected, collect(t, l))
			require.Equal(t, len(tc.expected) == 0, l.IsEmpty())

			for name, want := range tc.expected {
				got, ok := l.Get(name)
				require.True(t, ok, "Get(%q) should find the label", name)
				require.Equal(t, want, got, "Get(%q)", name)
				require.True(t, l.Has(name))
			}
		})
	}
}

func TestFromLabelSetMatchesFromMap(t *testing.T) {
	m := map[string]string{"zed": "z", "abc": "a", "mno": "", "b": "bb"}
	ls := commonlabels.LabelSet{}
	for k, v := range m {
		ls[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}

	fromMap, nMap := FromMap(m)
	fromLabelSet, nLabelSet := FromLabelSet(ls)

	require.Equal(t, nMap, nLabelSet)
	require.Equal(t, fromMap, fromLabelSet, "both constructors must produce identical bytes")
}

func TestFromPairsDeduplicatesKeepingLast(t *testing.T) {
	l, n := FromPairs([]Pair{
		{Name: "a", Value: "first"},
		{Name: "b", Value: "b"},
		{Name: "a", Value: "second"},
		{Name: "a", Value: "third"},
	})
	require.Equal(t, 2, n)
	require.Equal(t, map[string]string{"a": "third", "b": "b"}, collect(t, l))
}

func TestFromPairsDeduplicateThenDropEmpty(t *testing.T) {
	// The last occurrence wins, and if that last occurrence is empty the label is
	// dropped entirely rather than falling back to an earlier value.
	l, n := FromPairs([]Pair{
		{Name: "a", Value: "first"},
		{Name: "a", Value: ""},
	})
	require.Equal(t, 0, n)
	require.True(t, l.IsEmpty())
	_, ok := l.Get("a")
	require.False(t, ok)
}

func TestGetMissing(t *testing.T) {
	l, _ := FromMap(map[string]string{"bbb": "1", "ddd": "2", "fff": "3"})

	// Includes names that sort before, between and after the stored ones, plus
	// names sharing a first byte with a stored name, to cover every early-exit
	// branch in Get.
	for _, name := range []string{"", "a", "aaa", "b", "bb", "bbbb", "bbz", "ccc", "d", "ddz", "eee", "fffz", "g", "zzz"} {
		t.Run(fmt.Sprintf("name=%q", name), func(t *testing.T) {
			value, ok := l.Get(name)
			require.False(t, ok, "Get(%q) should not find a label", name)
			require.Empty(t, value)
			require.False(t, l.Has(name))
		})
	}
}

func TestGetEmptyLabels(t *testing.T) {
	value, ok := Empty.Get("anything")
	require.False(t, ok)
	require.Empty(t, value)
	require.Equal(t, 0, Empty.Len())
	require.True(t, Empty.IsEmpty())
	require.True(t, Empty.Range(func(string, string) bool { return true }))
}

// TestVarintBoundary covers the length encoding switchover: lengths 0-254 use a
// single byte, 255 and above use a 255 marker plus 3 bytes little-endian.
func TestVarintBoundary(t *testing.T) {
	for _, size := range []int{0, 1, 2, 253, 254, 255, 256, 257, 1000, 70000} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			long := strings.Repeat("x", size)

			// A long value, with a short name.
			if size > 0 {
				l, n := FromPairs([]Pair{{Name: "name", Value: long}})
				require.Equal(t, 1, n)
				got, ok := l.Get("name")
				require.True(t, ok)
				require.Equal(t, long, got)
				require.Equal(t, 1, l.Len())
			}

			// A long name, with a short value. Prefixed so the name is never
			// empty, which would collide with the "no labels" case.
			l, n := FromPairs([]Pair{{Name: "n" + long, Value: "value"}})
			require.Equal(t, 1, n)
			got, ok := l.Get("n" + long)
			require.True(t, ok)
			require.Equal(t, "value", got)
			require.Equal(t, 1, l.Len())
		})
	}
}

func TestEncodingMatchesPrometheus(t *testing.T) {
	// Our encoding must be byte-identical to the Prometheus stringlabels
	// encoding. Labels.Bytes exposes the raw buffer, so we can compare directly.
	// This test is meaningful only when Prometheus is built with the stringlabels
	// implementation, which is the default.
	if modellabels.ImplementationName != "stringlabels" {
		t.Skipf("prometheus labels implementation is %q, not stringlabels", modellabels.ImplementationName)
	}

	tests := []map[string]string{
		{},
		{"a": "1"},
		{"zed": "z", "abc": "a", "mno": "m", "b": "bb"},
		{"name": strings.Repeat("x", 254)},
		{"name": strings.Repeat("x", 255)},
		{"name": strings.Repeat("x", 256)},
		{strings.Repeat("n", 300): "value"},
	}

	for i, m := range tests {
		t.Run(fmt.Sprintf("case=%d", i), func(t *testing.T) {
			ours, _ := FromMap(m)

			promPairs := make([]modellabels.Label, 0, len(m))
			for _, name := range slices.Sorted(maps.Keys(m)) {
				promPairs = append(promPairs, modellabels.Label{Name: name, Value: m[name]})
			}
			theirs := modellabels.New(promPairs...)

			require.Equal(t, string(theirs.Bytes(nil)), string(ours))
		})
	}
}

func TestTooLongPanics(t *testing.T) {
	// Build a string longer than the 16MiB encoding limit. Constructed lazily so
	// the allocation only happens in this test.
	tooLong := strings.Repeat("x", maxLabelLen+1)

	require.PanicsWithValue(t, "labelpack: label too long to encode", func() {
		FromPairs([]Pair{{Name: "name", Value: tooLong}})
	})
	require.PanicsWithValue(t, "labelpack: label too long to encode", func() {
		FromPairs([]Pair{{Name: tooLong, Value: "value"}})
	})
}

func TestRangeEarlyExit(t *testing.T) {
	l, _ := FromMap(map[string]string{"a": "1", "b": "2", "c": "3"})

	var seen []string
	finished := l.Range(func(name, value string) bool {
		seen = append(seen, name)
		return name != "b"
	})
	require.False(t, finished, "Range must report that iteration was interrupted")
	require.Equal(t, []string{"a", "b"}, seen)

	finished = l.Range(func(name, value string) bool { return true })
	require.True(t, finished)
}

func TestIter(t *testing.T) {
	l, _ := FromMap(map[string]string{"b": "2", "a": "1"})
	it := l.Iter()

	name, value, ok := it.Next()
	require.True(t, ok)
	require.Equal(t, "a", name)
	require.Equal(t, "1", value)

	name, value, ok = it.Next()
	require.True(t, ok)
	require.Equal(t, "b", name)
	require.Equal(t, "2", value)

	_, _, ok = it.Next()
	require.False(t, ok)

	// Exhausted iterators keep reporting false.
	_, _, ok = it.Next()
	require.False(t, ok)
}

// smallSetSize is the stack-slice threshold in FromMap and FromLabelSet. Cross
// it to make sure both the stack and heap paths behave identically.
func TestLargeLabelSetCrossesStackThreshold(t *testing.T) {
	for _, count := range []int{smallSetSize - 1, smallSetSize, smallSetSize + 1, smallSetSize * 4} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			m := map[string]string{}
			for i := 0; i < count; i++ {
				m[fmt.Sprintf("label_%04d", i)] = fmt.Sprintf("value_%d", i)
			}

			l, n := FromMap(m)
			require.Equal(t, count, n)
			require.Equal(t, count, l.Len())
			require.Equal(t, m, collect(t, l))
		})
	}
}
