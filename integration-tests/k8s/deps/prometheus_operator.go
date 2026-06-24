package deps

import (
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const defaultPrometheusOperatorVersion = "v0.81.0"

// PrometheusOperator installs the upstream prometheus-operator bundle so
// tests can apply ServiceMonitor/PodMonitor/Probe/ScrapeConfig/etc CRs.
type PrometheusOperator struct {
	opts PrometheusOperatorOptions
}

type PrometheusOperatorOptions struct {
	// Version is the upstream prometheus-operator release tag (e.g. "v0.81.0").
	// When empty, defaultPrometheusOperatorVersion is used.
	Version string
}

func NewPrometheusOperator(opts PrometheusOperatorOptions) *PrometheusOperator {
	return &PrometheusOperator{opts: opts}
}

func (p *PrometheusOperator) Name() string { return "prometheus-operator" }

func (p *PrometheusOperator) Install(_ *harness.TestContext) error {
	v := p.opts.Version
	if v == "" {
		v = defaultPrometheusOperatorVersion
	}
	url := fmt.Sprintf(
		"https://github.com/prometheus-operator/prometheus-operator/releases/download/%s/bundle.yaml",
		v,
	)
	if err := harness.RunCommand("kubectl", "apply",
		"--server-side", "--validate=false", "-f", url,
	); err != nil {
		return fmt.Errorf("apply prometheus-operator bundle %s: %w", v, err)
	}
	// Wait for CRDs so later deps applying CRs don't race the apiserver
	// with "no matches for kind". --all is fine here: in a fresh kind
	// cluster the only CRDs present are the ones we just installed.
	if err := harness.RunCommand("kubectl", "wait",
		"--for=condition=established", "--timeout=2m", "crd", "--all",
	); err != nil {
		return fmt.Errorf("wait for prometheus-operator CRDs: %w", err)
	}
	return nil
}

// Cleanup is a no-op: CRDs are cluster-scoped, idempotent to re-apply, and
// torn down with the kind cluster.
func (p *PrometheusOperator) Cleanup() {}
