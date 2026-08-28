package alloycli

import (
	"io"
	"log/slog"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/stretchr/testify/require"
)

// descRe extracts the fqName and help text from prometheus.Desc.String().
var descRe = regexp.MustCompile(`fqName: "(.*?)", help: "(.*?)"`)

func helpByMetricName(c prometheus.Collector) map[string]string {
	ch := make(chan *prometheus.Desc, 32)
	go func() {
		c.Describe(ch)
		close(ch)
	}()

	out := make(map[string]string)
	for d := range ch {
		if m := descRe.FindStringSubmatch(d.String()); m != nil {
			out[m[1]] = m[2]
		}
	}
	return out
}

// TestResourcesCollectorHelpMatchesProcessCollector guards the shared
// alloy_resources_process_* metric family. beyla.ebpf registers a client_golang
// ProcessCollector under the "alloy_resources" namespace (see registerMetrics in
// the beyla component), so for any metric emitted by both collectors the help
// text must be identical — otherwise Prometheus fails /metrics gathering with an
// "inconsistent help" error (a 500). This caught a "unix" vs "Unix" epoch regression.
func TestResourcesCollectorHelpMatchesProcessCollector(t *testing.T) {
	ours := helpByMetricName(newResourcesCollector(slog.New(slog.NewTextHandler(io.Discard, nil))))
	processColl := helpByMetricName(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{
		Namespace: "alloy_resources",
	}))

	shared := 0
	for name, help := range processColl {
		if h, ok := ours[name]; ok {
			shared++
			require.Equalf(t, help, h, "help text for %q must match client_golang's ProcessCollector", name)
		}
	}
	require.GreaterOrEqual(t, shared, 3, "expected process_* metrics to overlap with client_golang's ProcessCollector")
}
