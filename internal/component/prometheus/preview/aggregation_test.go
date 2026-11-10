package preview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestAggregationBounds(t *testing.T) {
	args := testArguments()
	args.GroupBy = []string{"namespace"}
	args.MaxSeries = 10
	args.MaxGroups = 2
	a := newAggregation(args)
	for i := range 100 {
		a.observe(labels.FromStrings("__name__", "requests_total", "namespace", fmt.Sprintf("namespace-%d", i)), false)
	}
	require.Len(t, a.series, 10)
	require.Len(t, a.groups, 2)
	require.Equal(t, uint64(100), a.total.samples[0])
	require.Equal(t, uint64(10), a.total.series)
	require.True(t, a.total.limited)
	require.Equal(t, uint64(98), a.overflow.samples[0])
	require.Equal(t, uint64(8), a.overflow.series)
	require.True(t, a.overflow.limited)

	// Group admission is bounded over the entire observation, not per window.
	a.resetWindow()
	require.Empty(t, a.series)
	require.False(t, a.total.limited)
	require.Equal(t, uint64(0), a.total.series)
	require.Equal(t, uint64(100), a.total.samples[0])
	for i := range 100 {
		a.observe(labels.FromStrings("__name__", "new_metric", "namespace", fmt.Sprintf("new-%d", i)), true)
	}
	require.Len(t, a.groups, 2)
	require.Len(t, a.series, 10)
	require.Equal(t, uint64(100), a.overflow.samples[1])
}

func TestMergeBoundsAndDeduplication(t *testing.T) {
	args := testArguments()
	args.GroupBy = []string{"job"}
	args.MaxSeries = 3
	args.MaxGroups = 1
	a := newAggregation(args)
	for batch := range 3 {
		b := newAggregation(args)
		for i := range 5 {
			b.observe(labels.FromStrings("__name__", fmt.Sprintf("metric_%d", i), "job", fmt.Sprintf("job_%d", batch)), false)
		}
		a.merge(b)
	}
	require.Len(t, a.groups, 1)
	require.Len(t, a.series, 3)
	require.Equal(t, uint64(15), a.total.samples[0])
	require.True(t, a.total.limited)
	require.True(t, a.overflow.limited)
	require.Equal(t, uint64(10), a.overflow.samples[0])

	args.MaxSeries = 10
	b := newAggregation(args)
	ls := labels.FromStrings("__name__", "same", "job", "same")
	b.observe(ls, false)
	b.observe(ls, true)
	a = newAggregation(args)
	a.merge(b)
	a.merge(b)
	require.Equal(t, uint64(1), a.total.series)
	require.Equal(t, [2]uint64{2, 2}, a.total.samples)
	require.False(t, a.total.limited)
}

func TestGroupLabels(t *testing.T) {
	args := testArguments()
	args.GroupBy = []string{"job", "namespace"}
	for _, tc := range []struct {
		name     string
		ls       labels.Labels
		overflow bool
	}{
		{"selected only", labels.FromStrings("__name__", "private", "job", "api", "namespace", "prod", "secret", "hidden"), false},
		{"missing labels", labels.FromStrings("__name__", "private"), false},
		{"large value", labels.FromStrings("job", strings.Repeat("a", maxGroupValueBytes+1)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAggregation(args)
			a.observe(tc.ls, false)
			if tc.overflow {
				require.Empty(t, a.groups)
				require.Equal(t, uint64(1), a.overflow.series)
				return
			}
			require.Len(t, a.groups, 1)
			for _, g := range a.groups {
				require.Equal(t, labels.FromStrings("preview_group_job", tc.ls.Get("job"), "preview_group_namespace", tc.ls.Get("namespace")), g.labels)
			}
		})
	}
}

func TestOverflowAttributionIsMarkedAsLowerBound(t *testing.T) {
	args := testArguments()
	args.GroupBy = []string{"job"}
	args.MaxGroups = 1
	a := newAggregation(args)
	ls := labels.FromStrings("__name__", "metric", "job", "api")
	a.observe(ls, false)
	b := newAggregation(args)
	b.observe(labels.FromStrings("__name__", "other", "job", "other"), false)
	b.observe(ls, false) // This batch has no room for the already admitted api group.
	a.merge(b)
	require.Equal(t, uint64(2), a.total.series)
	require.False(t, a.total.limited)
	require.True(t, a.overflow.limited)
}

func BenchmarkObserve(b *testing.B) {
	args := testArguments()
	args.GroupBy = []string{"job"}
	a := newAggregation(args)
	ls := labels.FromStrings("__name__", "requests_total", "job", "api")
	b.ReportAllocs()
	for b.Loop() {
		a.observe(ls, false)
	}
}
