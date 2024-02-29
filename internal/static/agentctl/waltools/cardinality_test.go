package waltools

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCardinality(t *testing.T) {
	walDir := setupTestWAL(t)

	cardinality, err := FindCardinality(walDir, "test-job", "test-instance")
	sort.Slice(cardinality, func(i, j int) bool {
		return strings.Compare(cardinality[i].Metric, cardinality[j].Metric) == -1
	})

	require.NoError(t, err)
	require.Equal(t, []Cardinality{
		{Metric: "metric_0", Instances: 2},
		{Metric: "metric_1", Instances: 3}, // metric_1 has a duplicate hash so it's the only metric with 3 instances
		{Metric: "metric_2", Instances: 2},
		{Metric: "metric_3", Instances: 2},
		{Metric: "metric_4", Instances: 2},
		{Metric: "metric_5", Instances: 2},
		{Metric: "metric_6", Instances: 2},
		{Metric: "metric_7", Instances: 2},
		{Metric: "metric_8", Instances: 2},
		{Metric: "metric_9", Instances: 2},
	}, cardinality)
}
