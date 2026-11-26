package internal

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
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

	for _, tc := range []struct {
		name   string
		append func(ctx context.Context, a storage.AppendableV2) error
	}{
		{
			name: "V1",
			append: func(ctx context.Context, a storage.AppendableV2) error {
				app := a.(storage.Appendable).Appender(ctx)
				if _, err := app.Append(0, sampleLabels, ts, 1); err != nil {
					return err
				}
				return app.Commit()
			},
		},
		{
			name: "V2",
			append: func(ctx context.Context, a storage.AppendableV2) error {
				app := a.AppenderV2(ctx)
				if _, err := app.Append(0, sampleLabels, 0, ts, 1, nil, nil, storage.AppendV2Options{}); err != nil {
					return err
				}
				return app.Commit()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			a, err := NewAppendable(sink, receivertest.NewNopSettings(receivertest.NopType), false, nil, true, labels.EmptyLabels(), false)
			require.NoError(t, err)

			require.NoError(t, tc.append(scrapeCtx, a))

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			require.Equal(t, 1, mds[0].MetricCount())
		})
	}
}
