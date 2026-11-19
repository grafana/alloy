package deps

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/otel-gen.yaml
var otelGenManifest string

// The otel-gen:latest image is built and kind-loaded by the runner.
const (
	otelGenSelector            = "app=otel-gen"
	otelGenEndpointPlaceholder = "__OTEL_EXPORTER_ENDPOINT__"
)

// OtelGen is a tiny OTLP sender that pushes metrics of every type, and
// traces, over OTLP HTTP once a second.
type OtelGen struct {
	opts      OtelGenOptions
	manifest  string
	installed bool
}

type OtelGenOptions struct {
	Namespace string
	// Endpoint is the host:port of the OTLP HTTP receiver to send to, such as
	// "alloy-otel:4318".
	Endpoint string
}

func NewOtelGen(opts OtelGenOptions) *OtelGen {
	return &OtelGen{opts: opts}
}

func (o *OtelGen) Name() string { return "otel-gen" }

func (o *OtelGen) Install(_ *harness.TestContext) error {
	if o.opts.Namespace == "" {
		return fmt.Errorf("otel-gen namespace is required")
	}
	if o.opts.Endpoint == "" {
		return fmt.Errorf("otel-gen endpoint is required")
	}
	o.manifest = strings.ReplaceAll(otelGenManifest, otelGenEndpointPlaceholder, o.opts.Endpoint)
	if err := util.Step("apply otel-gen manifest", func() error {
		return harness.ApplyManifest(o.opts.Namespace, o.manifest)
	}); err != nil {
		return err
	}
	o.installed = true
	return util.Step("wait for otel-gen pod ready", func() error {
		return harness.WaitForReady(o.opts.Namespace, otelGenSelector)
	})
}

func (o *OtelGen) Cleanup() {
	if !o.installed {
		return
	}
	_ = harness.DeleteManifest(o.opts.Namespace, o.manifest)
}

// AssertOtelGenMetrics asserts that every metric otel-gen sends reached mimir
// under testName, each as the right kind of sample: floats with real data, and
// the exponential histograms as native histograms.
func AssertOtelGenMetrics(t *testing.T, mimir *Mimir, testName string) {
	t.Helper()
	floatMetrics := []string{
		"example_counter",
		"example_float_counter",
		"example_float_updowncounter",
		"example_histogram_bucket",
		"example_float_histogram_bucket",
	}
	// otel-gen only ever subtracts from this one, so its samples are negative
	// and it can't go through QueryPositive.
	negativeMetrics := []string{"example_updowncounter"}
	histogramMetrics := []string{
		"example_exponential_histogram",
		"example_exponential_float_histogram",
	}

	mimir.QueryMetrics(t, testName, append(append(append([]string{}, floatMetrics...), negativeMetrics...), histogramMetrics...))
	mimir.QueryPositive(t, testName, floatMetrics)
	mimir.QueryHistograms(t, testName, histogramMetrics, func(c *assert.CollectT, sample HistogramSample) {
		assert.Greaterf(c, sample.Count, 10.0, "%s: count", sample.Name())
		assert.Greaterf(c, sample.Sum, 10.0, "%s: sum", sample.Name())
		assert.NotEmptyf(c, sample.Buckets, "%s: buckets", sample.Name())
	})
}
