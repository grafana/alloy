package discovery

import (
	"reflect"
	"testing"
	"unsafe"

	commonlabels "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"
)

// TestTargetIsNotComparable guards the [0]func() field in Target.
//
// Now that labels are stored as a pointer and a string rather than as maps,
// Target would otherwise be comparable, and == would compile while being wrong:
// two targets holding identical labels can reference different groupLabels, and
// two targets with the same labels split differently between group and own have
// different field values. Callers must use EqualsTarget.
//
// This also keeps Target out of map keys, which the compiler previously
// prevented.
func TestTargetIsNotComparable(t *testing.T) {
	require.False(t, reflect.TypeOf(Target{}).Comparable(),
		"Target must not be comparable; keep the [0]func() field so that == and map keys stay compile errors")
}

// TestTargetSize documents the size of a Target, since holding hundreds of
// thousands of them is a normal workload. It is a pointer, a string header and an
// int32, so 8+16+4 rounded up to the alignment of a pointer.
//
// The [0]func() field must stay first: a zero-size field at the end of a struct
// gets trailing padding added, which would make Target larger for no reason.
func TestTargetSize(t *testing.T) {
	require.Equal(t, uintptr(32), unsafe.Sizeof(Target{}),
		"unexpected Target size; ensure the zero-size field stays first so it adds no padding")
}

// TestZeroValueTargetIsUsable checks that the zero value behaves as an empty
// target. Code outside this package constructs Target{} and passes it to
// NewTargetBuilderFrom, so a nil group and an empty packed buffer must be valid
// everywhere.
func TestZeroValueTargetIsUsable(t *testing.T) {
	var zero Target

	require.Equal(t, 0, zero.Len())
	require.Equal(t, "{}", zero.String())
	require.Empty(t, zero.AsMap())
	require.Empty(t, zero.LabelSet())
	require.Empty(t, zero.NonReservedLabelSet())
	require.Equal(t, 0, zero.PromLabels().Len())

	value, ok := zero.Get("anything")
	require.False(t, ok)
	require.Empty(t, value)

	require.True(t, zero.ForEachLabel(func(string, string) bool {
		t.Fatal("the zero value must have no labels")
		return true
	}))

	// EmptyTarget is documented as equivalent to the zero value.
	require.True(t, zero.EqualsTarget(&EmptyTarget))
	require.True(t, EmptyTarget.EqualsTarget(&zero))

	// Hashing an empty target must not panic and must be stable.
	require.Equal(t, EmptyTarget.NonMetaLabelsHash(), zero.NonMetaLabelsHash())

	// A builder over the zero value must work, and must be able to add labels.
	built := NewTargetBuilderFrom(zero)
	built.Set("foo", "bar")
	result := built.Target()
	require.Equal(t, 1, result.Len())
	value, ok = result.Get("foo")
	require.True(t, ok)
	require.Equal(t, "bar", value)
}

// TestGroupLabelsSharedAcrossTargets checks that targets discovered together
// share one copy of their group labels. This is the property that keeps memory
// flat when a single service discovery group yields many targets, and losing it
// would be an invisible regression.
func TestGroupLabelsSharedAcrossTargets(t *testing.T) {
	cache := targetGroupCacheForTest(t)
	targets := toAlloyTargets(cache)
	require.Greater(t, len(targets), 1)

	first := targets[0].group
	require.NotNil(t, first, "targets should have group labels")
	for i, target := range targets {
		require.Same(t, first, target.group,
			"target %d must share the group labels pointer with the other targets of its group", i)
	}

	// Relabeling that does not touch group labels must preserve the sharing.
	for i := range targets {
		tb := NewTargetBuilderFrom(targets[i])
		tb.Set("added", "value")
		relabelled := tb.Target()
		require.Same(t, first, relabelled.group,
			"changing own labels must not copy the group labels")
	}

	// Deleting a group label has to narrow the group for that target only.
	tb := NewTargetBuilderFrom(targets[0])
	tb.Del("shared")
	narrowed := tb.Target()
	require.NotSame(t, first, narrowed.group)
	_, ok := narrowed.Get("shared")
	require.False(t, ok, "deleted group label must not shine through")
	// The other targets keep the original group untouched.
	value, ok := targets[1].Get("shared")
	require.True(t, ok)
	require.Equal(t, "shared_value", value)
}

// targetGroupCacheForTest builds a service discovery cache with one group whose
// targets share group labels.
func targetGroupCacheForTest(t *testing.T) map[string]*targetgroup.Group {
	t.Helper()
	return map[string]*targetgroup.Group{
		"source_a": {
			Source: "source_a",
			Labels: commonlabels.LabelSet{"shared": "shared_value", "job": "test"},
			Targets: []commonlabels.LabelSet{
				{"__address__": "10.0.0.1:80"},
				{"__address__": "10.0.0.2:80"},
				{"__address__": "10.0.0.3:80"},
			},
		},
	}
}
