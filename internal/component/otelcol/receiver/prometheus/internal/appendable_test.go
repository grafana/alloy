package internal

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/grafana/alloy/internal/util/testappender"
)

// TestAppendableAppenders checks that a sample appended through each appender
// version is emitted to the sink.
func TestAppendableAppenders(t *testing.T) {
	sampleLabels := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	})
	ts := time.Now().UnixMilli()
	appends := testappender.Appends{
		V1: func(app storage.Appender) error {
			_, err := app.Append(0, sampleLabels, ts, 1)
			return err
		},
		V2: func(app storage.AppenderV2) error {
			_, err := app.Append(0, sampleLabels, 0, ts, 1, nil, nil, storage.AppendV2Options{})
			return err
		},
	}

	for _, version := range testappender.Versions {
		t.Run(version, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			a, err := NewAppendable(sink, receivertest.NewNopSettings(receivertest.NopType), false, nil, true, labels.EmptyLabels(), false)
			require.NoError(t, err)

			require.NoError(t, appends.AppendAndCommit(scrapeCtx, version, a))

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			require.Equal(t, 1, mds[0].MetricCount())
		})
	}
}
