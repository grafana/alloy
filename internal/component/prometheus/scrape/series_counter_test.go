package scrape

import (
	"context"
	"strconv"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util/testappender"
)

// The series counter attributes each scrape's series to the target's clustering
// key, read from the stamped ClusteringKeyMetaLabel via the appender context.
func TestSeriesCounter_CountsPerTargetKey(t *testing.T) {
	const key uint64 = 0xABCDEF

	sc := newSeriesCounter(testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})

	// Build a target whose discovered labels carry the stamped clustering key,
	// exactly as the scrape component stamps it, and put it in the context like
	// Prometheus' scrape loop does.
	tLabels := model.LabelSet{
		"__address__":                    "10.0.0.1:9100",
		discovery.ClusteringKeyMetaLabel: model.LabelValue(strconv.FormatUint(key, 10)),
	}
	target := scrape.NewTarget(labels.EmptyLabels(), &config.ScrapeConfig{}, tLabels, nil)
	ctx := scrape.ContextWithTarget(context.Background(), target)

	app := sc.Appender(ctx)
	for i := 0; i < 5; i++ {
		_, err := app.Append(0, labels.FromStrings("__name__", "m", "i", strconv.Itoa(i)), 1, float64(i))
		require.NoError(t, err)
	}
	require.NoError(t, app.Commit())

	require.Equal(t, uint64(5), sc.Weights()[key], "should record 5 series for the target's key")

	// A second scrape with fewer series replaces the count (latest wins).
	app = sc.Appender(ctx)
	_, err := app.Append(0, labels.FromStrings("__name__", "m", "i", "0"), 1, 0)
	require.NoError(t, err)
	require.NoError(t, app.Commit())
	require.Equal(t, uint64(1), sc.Weights()[key])
}

// Without a stamped key (clustering off / non-clustered target), the counter is a
// transparent pass-through and records nothing.
func TestSeriesCounter_NoKeyIsPassThrough(t *testing.T) {
	sc := newSeriesCounter(testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})

	target := scrape.NewTarget(labels.EmptyLabels(), &config.ScrapeConfig{}, model.LabelSet{"__address__": "10.0.0.2:9100"}, nil)
	ctx := scrape.ContextWithTarget(context.Background(), target)

	app := sc.Appender(ctx)
	_, err := app.Append(0, labels.FromStrings("__name__", "m"), 1, 1)
	require.NoError(t, err)
	require.NoError(t, app.Commit())

	require.Empty(t, sc.Weights(), "no clustering key stamped -> nothing recorded")
}
