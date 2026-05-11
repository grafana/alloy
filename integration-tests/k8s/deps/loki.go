package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// Both must match manifests/loki.yaml.
	lokiSelector = "app=loki"
	lokiHTTPPort = "3100"
)

//go:embed manifests/loki.yaml
var lokiManifest string

// Compile-time check that *Loki satisfies the harness.Dependency interface.
var _ harness.Dependency = (*Loki)(nil)

// Loki runs a single-pod Loki in monolithic mode (filesystem storage,
// in-memory rings). In-cluster URL: http://loki:3100.
type Loki struct {
	opts            LokiOptions
	namespace       string
	localPort       string
	stopPortForward func()
	installed       bool
}

type LokiOptions struct {
	Namespace string
}

func NewLoki(opts LokiOptions) *Loki {
	return &Loki{opts: opts, namespace: opts.Namespace}
}

func (l *Loki) Name() string { return "loki" }

func (l *Loki) Install(ctx *harness.TestContext) error {
	if l.namespace == "" {
		return fmt.Errorf("loki namespace is required")
	}

	if err := util.Step("apply loki manifest", func() error {
		return harness.ApplyManifest(l.namespace, lokiManifest)
	}); err != nil {
		return err
	}
	l.installed = true

	if err := util.Step("wait for loki pod ready", func() error {
		return harness.WaitForReady(l.namespace, lokiSelector)
	}); err != nil {
		return err
	}

	localPort, stop, err := startPortForwardWithRetries(l.namespace, 5, lokiHTTPPort)
	if err != nil {
		return err
	}
	l.localPort = localPort
	l.stopPortForward = stop
	return nil
}

func (l *Loki) Cleanup() {
	if l.stopPortForward != nil {
		l.stopPortForward()
	}
	if !l.installed || l.namespace == "" {
		return
	}
	_ = harness.DeleteManifest(l.namespace, lokiManifest)
}

// endpoint builds an absolute URL against the local port-forward.
func (l *Loki) endpoint(path string) string {
	return "http://localhost:" + l.localPort + path
}
