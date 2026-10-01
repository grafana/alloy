package scrape

import (
	"errors"
	"testing"
	"time"

	commonmodel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	promscrape "github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
)

func TestBuildComponentTargetsSnapshotsScrapeManagerState(t *testing.T) {
	attempt := time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC)
	scrapeError := errors.New("connection refused")
	config := promconfig.DefaultScrapeConfig
	target := promscrape.NewTarget(
		labels.FromStrings(
			commonmodel.SchemeLabel, "https",
			commonmodel.AddressLabel, "example.com:9090",
			commonmodel.MetricsPathLabel, "/metrics",
			"job", "api",
		),
		&config,
		commonmodel.LabelSet{
			"__address__":   "example.com:9090",
			"__meta_source": "kubernetes",
		},
		nil,
	)
	target.Report(attempt, 250*time.Millisecond, scrapeError)

	got := buildComponentTargets(map[string][]*promscrape.Target{
		"prometheus.scrape.api": {target},
	})

	require.Len(t, got, 1)
	require.Equal(t, "api", got[0].NonMetaLabels["job"])
	require.Equal(t, "kubernetes", got[0].Labels["__meta_source"])
	require.NotZero(t, got[0].Hash)
	require.NotZero(t, got[0].NonMetaHash)
	require.Equal(t, &component.ScrapeTargetInfo{
		URL:          "https://example.com:9090/metrics",
		Health:       component.ScrapeTargetHealthDown,
		LastAttempt:  attempt,
		LastDuration: 250 * time.Millisecond,
		LastError:    scrapeError,
	}, got[0].Scrape)
}
