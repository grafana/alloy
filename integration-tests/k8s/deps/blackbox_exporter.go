package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/blackbox-exporter.yaml
var blackboxExporterManifest string

const (
	blackboxExporterImage    = "prom/blackbox-exporter:v0.25.0"
	blackboxExporterSelector = "app=blackbox-exporter"
)

// BlackboxExporter installs prom/blackbox-exporter as a Probe target.
type BlackboxExporter struct {
	opts      BlackboxExporterOptions
	installed bool
}

type BlackboxExporterOptions struct {
	Namespace string
}

func NewBlackboxExporter(opts BlackboxExporterOptions) *BlackboxExporter {
	return &BlackboxExporter{opts: opts}
}

func (b *BlackboxExporter) Name() string { return "blackbox-exporter" }

func (b *BlackboxExporter) Install(ctx *harness.TestContext) error {
	if b.opts.Namespace == "" {
		return fmt.Errorf("blackbox-exporter namespace is required")
	}
	if err := ensureKindImage(blackboxExporterImage); err != nil {
		return err
	}
	if err := util.Step("apply blackbox-exporter manifest", func() error {
		return harness.ApplyManifest(b.opts.Namespace, blackboxExporterManifest)
	}); err != nil {
		return err
	}
	b.installed = true
	return util.Step("wait for blackbox-exporter pod ready", func() error {
		return harness.WaitForReady(b.opts.Namespace, blackboxExporterSelector)
	})
}

func (b *BlackboxExporter) Cleanup() {
	if !b.installed {
		return
	}
	_ = harness.DeleteManifest(b.opts.Namespace, blackboxExporterManifest)
}
