package scrape

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	googlev1 "github.com/grafana/pyroscope/api/gen/proto/go/google/v1"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestDeltaProfilesManager(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	reloadInterval = time.Millisecond
	defer func() {
		reloadInterval = 5 * time.Second
	}()
	var servers []*httptest.Server
	var serverAddresses []string
	p1 := newMemoryProfile(0, (15 * time.Second).Nanoseconds())
	p2 := newMemoryProfile(int64(15*time.Second), (15 * time.Second).Nanoseconds())
	p2.Sample[0].Value[0] += 10

	for i := range []int{0, 1, 2} {
		server := httptest.NewServer(&sliceProfileHandler{
			t: t,
			profiles: map[string][]*googlev1.Profile{
				"/debug/pprof/allocs": {p1, p1, p2},
			},
		})

		t.Logf("Created server %d for  at %s", i, server.URL)
		servers = append(servers, server)
		serverAddresses = append(serverAddresses, server.URL)
	}

	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	registry := prometheus.NewRegistry()
	appendedProfiles := make(chan *pyroscope.RawSample, 100)
	logger := util.TestLogger(t)
	logger = level.NewFilter(logger, level.AllowAll())
	m := NewManager(Options{}, pyroscope.AppendableFunc(func(ctx context.Context, lbs labels.Labels, samples []*pyroscope.RawSample) error {
		t.Logf("Appending %d samples with labels: %v", len(samples), lbs)
		for _, sample := range samples {
			select {
			case appendedProfiles <- sample:
			default:
				t.Logf("Failed to send sample to channel")
			}
		}
		return nil
	}), logger, registry)
	defer m.Stop()

	a := NewDefaultArguments()
	a.ScrapeInterval = 1 * time.Second
	a.ProfilingConfig.ProcessCPU.Enabled = false
	a.ProfilingConfig.Goroutine.Enabled = false
	a.ProfilingConfig.Block.Enabled = false
	a.ProfilingConfig.Mutex.Enabled = false
	require.NoError(t, m.ApplyConfig(a))

	targetSetsChan := make(chan map[string][]*targetgroup.Group)
	go m.Run(targetSetsChan)

	var targets []model.LabelSet
	for i, serverURL := range serverAddresses {
		targets = append(targets, model.LabelSet{
			model.AddressLabel: model.LabelValue(strings.TrimPrefix(serverURL, "http://")),
			serviceNameLabel:   model.LabelValue(fmt.Sprintf("service-%d", i)),
			"namespace":        model.LabelValue(fmt.Sprintf("namespace-%d", i)),
		})
	}

	targetSetsChan <- map[string][]*targetgroup.Group{"profile_group": {{Targets: targets}}}

	require.Eventually(t, func() bool {
		gs := m.TargetsActive()
		t.Logf("Active targets: %v", gs)
		return len(gs["profile_group"]) == 3
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		count := len(appendedProfiles)
		expected := len(serverAddresses) * 2
		return count == expected
	}, 15*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		metrics, err := registry.Gather()
		if err != nil {
			t.Logf("Error gathering metrics: %v", err)
			return false
		}

		for _, metric := range metrics {
			if metric.GetName() == "pyroscope_delta_map_size" {
				return len(metric.GetMetric()) > 0
			}
		}
		t.Logf("pyroscope_delta_map_size metric not found")
		return false
	}, 2*time.Second, 100*time.Millisecond)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	var deltaMapSizeMetric *dto.MetricFamily
	for _, mf := range metricFamilies {
		if mf.GetName() == "pyroscope_delta_map_size" {
			deltaMapSizeMetric = mf
			break
		}
	}
	require.NotNil(t, deltaMapSizeMetric, "pyroscope_delta_map_size metric not found")

	expected := map[string]float64{
		"namespace-0": 1.0,
		"namespace-1": 1.0,
		"namespace-2": 1.0,
	}
	actual := map[string]float64{}
	for _, m := range deltaMapSizeMetric.GetMetric() {
		for _, label := range m.GetLabel() {
			ns := label.GetValue()
			if label.GetName() == "aggregation" {
				v := m.GetGauge().GetValue()
				actual[ns] = v
				require.Greater(t, v, 0.0, "Delta map size should be greater than 0")
				t.Logf("Delta map size : %v %v", ns, v)
			}
		}
	}
	require.Equal(t, expected, actual, "Delta map size metrics should match expected values")
}

type sliceProfileHandler struct {
	t         *testing.T
	profileMu sync.Mutex
	profiles  map[string][]*googlev1.Profile
}

func (d *sliceProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.profileMu.Lock()
	defer d.profileMu.Unlock()

	u := r.URL
	ps := d.profiles[u.Path]
	if len(ps) == 0 {
		w.WriteHeader(404)
		return
	}
	p := ps[0]
	d.profiles[u.Path] = ps[1:]

	data := marshal(d.t, p)
	w.Write(data)
}
