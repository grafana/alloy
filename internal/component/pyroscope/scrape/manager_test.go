package scrape

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestManager(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	reloadInterval = time.Millisecond

	m, _ := NewManager(Options{}, NewDefaultArguments(), pyroscope.AppendableFunc(func(ctx context.Context, labels labels.Labels, samples []*pyroscope.RawSample) error {
		return nil
	}), util.TestLogger(t))

	defer m.Stop()
	targetSetsChan := make(chan []*targetgroup.Group)
	require.NoError(t, m.ApplyConfig(NewDefaultArguments()))
	go m.Run(targetSetsChan)

	targetSetsChan <- []*targetgroup.Group{
		{
			Targets: []model.LabelSet{
				{model.AddressLabel: "localhost:9090", serviceNameLabel: "s"},
				{model.AddressLabel: "localhost:8080", serviceNameK8SLabel: "k"},
			},
			Labels: model.LabelSet{"foo": "bar"},
		},
	}
	require.Eventually(t, func() bool {
		return len(m.TargetsActive()) == 10
	}, time.Second, 10*time.Millisecond)

	new := NewDefaultArguments()
	new.ScrapeInterval = 1 * time.Second

	// Trigger a config reload
	require.NoError(t, m.ApplyConfig(new))

	targetSetsChan <- []*targetgroup.Group{
		{
			Targets: []model.LabelSet{
				{model.AddressLabel: "localhost:9090", serviceNameLabel: "s"},
				{model.AddressLabel: "localhost:8080", serviceNameK8SLabel: "k"},
				{model.AddressLabel: "localhost:8081", serviceNameK8SLabel: "k2"},
			},
			Labels: model.LabelSet{"foo": "bar"},
		},
	}

	require.Eventually(t, func() bool {
		return len(m.TargetsActive()) == 15
	}, time.Second, 10*time.Millisecond)

	require.Equal(t, 1*time.Second, m.sp.config.ScrapeInterval)

	targetSetsChan <- []*targetgroup.Group{}

	require.Eventually(t, func() bool {
		return len(m.TargetsAll()) == 0
	}, time.Second, 10*time.Millisecond)
}
