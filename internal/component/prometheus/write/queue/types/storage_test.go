package types

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	ts := GetTimeSeriesFromPool()
	ts.Labels = labels.FromStrings("one", "two")
	ts.LabelsValues = make([]uint32, 1)
	ts.LabelsNames = make([]uint32, 1)
	ts.LabelsValues[0] = 1
	ts.LabelsNames[0] = 2

	PutTimeSeriesIntoPool(ts)
	ts = GetTimeSeriesFromPool()
	defer PutTimeSeriesIntoPool(ts)
	require.Len(t, ts.Labels, 0)
	require.Len(t, ts.LabelsValues, 0)
	require.Len(t, ts.LabelsNames, 0)
}
