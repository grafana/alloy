package file

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
)

func TestDuplicateDetectionWithMultipleDifferingLabels(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))

	var logBuf bytes.Buffer
	testLogger := log.NewLogfmtLogger(&logBuf)

	registry := prometheus.NewRegistry()

	opts := component.Options{
		Logger:        testLogger,
		Registerer:    registry,
		OnStateChange: func(e component.Exports) {},
		DataPath:      t.TempDir(),
	}

	f, err := os.CreateTemp(opts.DataPath, "multi_label_test")
	require.NoError(t, err)
	defer f.Close()

	ch := loki.NewLogsReceiver()

	// Create two targets with multiple differing labels using Alloy syntax.
	config := fmt.Sprintf(`
		forward_to = []
		targets = [
			{__path__ = %q, app = "myapp", port = "8080", container = "main"},
			{__path__ = %q, app = "myapp", port = "9090", container = "sidecar"},
		]
	`, f.Name(), f.Name())

	var args Arguments
	err = syntax.Unmarshal([]byte(config), &args)
	require.NoError(t, err)

	// Set the actual receiver after parsing.
	args.ForwardTo = []loki.LogsReceiver{ch}

	c, err := New(opts, args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Verify that a warning was logged.
	logOutput := logBuf.String()
	require.Contains(t, logOutput, "file has multiple targets with different labels which will cause duplicate log lines")

	// Both differing labels should be mentioned.
	require.True(t, strings.Contains(logOutput, `differing_labels="container, port"`),
		"expected both differing labels to be mentioned in log, got: %s", logOutput)

	// Verify that the metric is set.
	metricValue := testutil.ToFloat64(c.metrics.duplicateFilesTally)
	require.Equal(t, float64(1), metricValue)
}
