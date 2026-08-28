package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/prom-gen.yaml
var promGenManifest string

// The prom-gen:latest image is built and kind-loaded by the runner.
const promGenSelector = "app=prom-gen"

// PromGen is a tiny HTTP server emitting Prometheus metrics. Used as a
// scrape target in prometheus-operator tests.
type PromGen struct {
	opts      PromGenOptions
	installed bool
}

type PromGenOptions struct {
	Namespace string
}

func NewPromGen(opts PromGenOptions) *PromGen {
	return &PromGen{opts: opts}
}

func (p *PromGen) Name() string { return "prom-gen" }

func (p *PromGen) Install(_ *harness.TestContext) error {
	if p.opts.Namespace == "" {
		return fmt.Errorf("prom-gen namespace is required")
	}
	if err := util.Step("apply prom-gen manifest", func() error {
		return harness.ApplyManifest(p.opts.Namespace, promGenManifest)
	}); err != nil {
		return err
	}
	p.installed = true
	return util.Step("wait for prom-gen pod ready", func() error {
		return harness.WaitForReady(p.opts.Namespace, promGenSelector)
	})
}

func (p *PromGen) Cleanup() {
	if !p.installed {
		return
	}
	_ = harness.DeleteManifest(p.opts.Namespace, promGenManifest)
}
