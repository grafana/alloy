package labelpack

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// mergeToMap drains a MergeIter into a map, asserting ascending name order and
// that no name is yielded twice.
func mergeToMap(t *testing.T, group, own Labels) map[string]string {
	t.Helper()
	out := map[string]string{}
	it := Merge(group, own)
	prev := ""
	first := true
	for {
		name, value, ok := it.Next()
		if !ok {
			return out
		}
		if !first {
			require.Less(t, prev, name, "merged labels must be yielded in ascending name order")
		}
		prev, first = name, false
		_, dup := out[name]
		require.False(t, dup, "label %q yielded twice", name)
		out[name] = value
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		group    map[string]string
		own      map[string]string
		expected map[string]string
	}{
		{
			name:     "both empty",
			expected: map[string]string{},
		},
		{
			name:     "only group",
			group:    map[string]string{"a": "1", "b": "2"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "only own",
			own:      map[string]string{"a": "1", "b": "2"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "disjoint, interleaved",
			group:    map[string]string{"b": "gb", "d": "gd"},
			own:      map[string]string{"a": "oa", "c": "oc", "e": "oe"},
			expected: map[string]string{"a": "oa", "b": "gb", "c": "oc", "d": "gd", "e": "oe"},
		},
		{
			name:     "own shadows group",
			group:    map[string]string{"a": "group", "b": "gb"},
			own:      map[string]string{"a": "own"},
			expected: map[string]string{"a": "own", "b": "gb"},
		},
		{
			name:     "own shadows every group label",
			group:    map[string]string{"a": "ga", "b": "gb"},
			own:      map[string]string{"a": "oa", "b": "ob"},
			expected: map[string]string{"a": "oa", "b": "ob"},
		},
		{
			name:     "group entirely before own",
			group:    map[string]string{"a": "ga", "b": "gb"},
			own:      map[string]string{"y": "oy", "z": "oz"},
			expected: map[string]string{"a": "ga", "b": "gb", "y": "oy", "z": "oz"},
		},
		{
			name:     "own entirely before group",
			group:    map[string]string{"y": "gy", "z": "gz"},
			own:      map[string]string{"a": "oa", "b": "ob"},
			expected: map[string]string{"a": "oa", "b": "ob", "y": "gy", "z": "gz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			group, _ := FromMap(tc.group)
			own, _ := FromMap(tc.own)

			require.Equal(t, tc.expected, mergeToMap(t, group, own), "Merge")
			require.Equal(t, len(tc.expected), MergedLen(group, own), "MergedLen")

			// RangeMerged has fast paths for an empty side; it must agree with the
			// general merge either way.
			viaRange := map[string]string{}
			finished := RangeMerged(group, own, func(name, value string) bool {
				viaRange[name] = value
				return true
			})
			require.True(t, finished)
			require.Equal(t, tc.expected, viaRange, "RangeMerged")
		})
	}
}

func TestRangeMergedEarlyExit(t *testing.T) {
	group, _ := FromMap(map[string]string{"b": "gb", "d": "gd"})
	own, _ := FromMap(map[string]string{"a": "oa", "c": "oc"})

	var seen []string
	finished := RangeMerged(group, own, func(name, value string) bool {
		seen = append(seen, name)
		return name != "b"
	})
	require.False(t, finished, "RangeMerged must report that iteration was interrupted")
	require.Equal(t, []string{"a", "b"}, seen)
}

func TestRangeMergedEarlyExitSingleSided(t *testing.T) {
	// Exercise the empty-side fast paths, which delegate to Labels.Range.
	labels, _ := FromMap(map[string]string{"a": "1", "b": "2", "c": "3"})

	for _, tc := range []struct {
		name       string
		group, own Labels
	}{
		{name: "empty group", group: Empty, own: labels},
		{name: "empty own", group: labels, own: Empty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			finished := RangeMerged(tc.group, tc.own, func(name, value string) bool {
				seen = append(seen, name)
				return name != "a"
			})
			require.False(t, finished)
			require.Equal(t, []string{"a"}, seen)
		})
	}
}

func TestMergedLenLargeSets(t *testing.T) {
	// Overlapping ranges so that shadowed labels are counted once.
	groupMap := map[string]string{}
	ownMap := map[string]string{}
	for i := 0; i < 100; i++ {
		groupMap[fmt.Sprintf("label_%04d", i)] = "group"
	}
	for i := 50; i < 200; i++ {
		ownMap[fmt.Sprintf("label_%04d", i)] = "own"
	}

	group, _ := FromMap(groupMap)
	own, _ := FromMap(ownMap)

	require.Equal(t, 200, MergedLen(group, own))

	merged := mergeToMap(t, group, own)
	require.Len(t, merged, 200)
	require.Equal(t, "group", merged["label_0000"])
	require.Equal(t, "own", merged["label_0050"], "own must shadow group")
	require.Equal(t, "own", merged["label_0199"])
}
