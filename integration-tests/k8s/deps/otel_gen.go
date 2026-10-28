package deps

import (
	_ "embed"
	"fmt"
	"strings"

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
