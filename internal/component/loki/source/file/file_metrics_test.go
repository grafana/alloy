package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// fileMetricSnapshot gathers rather than calling WithLabelValues: asking a GaugeVec for a child would create the
// missing path-series and hide the bug. The path gauge is reader presence, including a quiet file that has never
// produced a complete line.
func fileMetricSnapshot(reg *prometheus.Registry) (map[string]float64, float64, error) {
	families, err := reg.Gather()
	if err != nil {
		return nil, 0, err
	}
	paths := make(map[string]float64)
	var active float64
	for _, family := range families {
		switch family.GetName() {
		case "loki_source_file_files_active_total":
			active = family.Metric[0].GetGauge().GetValue()
		case "loki_source_file_file_bytes_total":
			for _, metric := range family.Metric {
				for _, label := range metric.Label {
					if label.GetName() == "path" {
						paths[label.GetValue()] = metric.GetGauge().GetValue()
					}
				}
			}
		}
	}
	return paths, active, nil
}

func runMetricTailer(t *testing.T, tailer *tailer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tailer.Run(ctx, ctx)
	}()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("file tailer did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
}

func requireFileMetric(t *testing.T, reg *prometheus.Registry, path string, size, active float64, present bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		paths, count, err := fileMetricSnapshot(reg)
		value, found := paths[path]
		return err == nil && count == active && found == present && (!present || value == size)
	}, 5*time.Second, 5*time.Millisecond)
}

func testFileWatch() FileWatch {
	return FileWatch{
		MinPollFrequency: 25 * time.Millisecond,
		MaxPollFrequency: 25 * time.Millisecond,
	}
}

func TestFileMetricsIdleReaderLifecycle(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	for _, content := range []string{"", "not yet a line"} {
		t.Run(content, func(t *testing.T) {
			runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, _ func() []loki.Entry) {
				path := filepath.Join(t.TempDir(), "quiet.log")
				require.NoError(t, os.WriteFile(path, []byte(content), 0600))
				logger := logging.NewSlogNop()
				pos, err := positions.New(logger, positions.Config{
					Path: filepath.Join(t.TempDir(), "positions.yaml"),
				})
				require.NoError(t, err)
				t.Cleanup(pos.Stop)
				reg := prometheus.NewRegistry()
				m := newMetrics(reg)
				labels := model.LabelSet{"source": "quiet"}
				tailer := newTailer(m, logger, newBatcher, pos, func() bool { return false }, sourceOptions{
					path:                 path,
					labels:               labels,
					fileWatch:            testFileWatch(),
					onPositionsFileError: OnPositionsFileErrorRestartBeginning,
				})

				// The scheduler's retry invokes Run again on this same source, without running the component's
				// source factory again.
				for run := 0; run < 2; run++ {
					stop := runMetricTailer(t, tailer)
					requireFileMetric(t, reg, path, float64(len(content)), 1, true)
					// Publishing reader presence must not advance the read offset past the unread
					// content; the tailer itself persists its zero offset when it stops.
					require.Contains(t, []string{"", "0"}, pos.GetString(path, labels.String()))
					stop()
					requireFileMetric(t, reg, path, 0, 0, false)
					require.Empty(t, m.activePaths)
				}
			})
		})
	}
}

// staticPipeline satisfies the loki pipeline service's Data for a component constructed outside a controller.
type staticPipeline bool

func (p staticPipeline) PipelineV2Enabled() bool { return bool(p) }

func TestFileMetricsFailedReaderNeverPublishes(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	path := filepath.Join(t.TempDir(), "unreadable.log")
	require.NoError(t, os.WriteFile(path, nil, 0600))
	reg := prometheus.NewRegistry()
	args := Arguments{}
	args.SetToDefault()
	args.Encoding = "not-a-supported-encoding"
	args.Targets = []discovery.Target{discovery.NewTargetFromMap(map[string]string{"__path__": path})}
	c, err := New(component.Options{
		Logger: logging.NewSlogNop(), Registerer: reg, DataPath: t.TempDir(),
		GetServiceData: func(string) (any, error) { return staticPipeline(false), nil },
	}, args)
	require.NoError(t, err)
	// Stopped inline, not via t.Cleanup: the goleak defer above runs before cleanups would.
	// Stop joins any attempt that started. Stat and target discovery succeeded, but opening the actual
	// reader cannot.
	c.scheduler.Stop()
	c.watcher.Stop()
	c.posFile.Stop()
	requireFileMetric(t, reg, path, 0, 0, false)
}

func TestFileMetricsOverlappingReaderCleanup(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, _ func() []loki.Entry) {
		path := filepath.Join(t.TempDir(), "replacement.log")
		require.NoError(t, os.WriteFile(path, nil, 0600))
		reg := prometheus.NewRegistry()
		m := newMetrics(reg)
		makeTailer := func(label string) *tailer {
			return newTailer(m, logging.NewSlogNop(), newBatcher, positions.NewNop(), func() bool { return true }, sourceOptions{
				path:                 path,
				labels:               model.LabelSet{"generation": model.LabelValue(label)},
				fileWatch:            testFileWatch(),
				onPositionsFileError: OnPositionsFileErrorRestartBeginning,
			})
		}
		stopOld := runMetricTailer(t, makeTailer("old"))
		requireFileMetric(t, reg, path, 0, 1, true)
		// Reconciliation starts the replacement before canceling the old labels, and stopping a source does
		// not join the old reader.
		stopNew := runMetricTailer(t, makeTailer("new"))
		requireFileMetric(t, reg, path, 0, 2, true)
		stopOld()
		requireFileMetric(t, reg, path, 0, 1, true)
		stopNew()
		requireFileMetric(t, reg, path, 0, 0, false)
		require.Empty(t, m.activePaths)
	})
}
