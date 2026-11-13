package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/moto.yaml
var motoManifest string

const motoSelector = "app=moto"

var _ harness.Dependency = (*Moto)(nil)

// Moto installs a moto server as a fake AWS API. A seed Job creates the
// secret that the remote.aws.secrets_manager test reads.
type Moto struct {
	opts      MotoOptions
	installed bool
}

type MotoOptions struct {
	Namespace string
}

func NewMoto(opts MotoOptions) *Moto {
	return &Moto{opts: opts}
}

func (m *Moto) Name() string { return "moto" }

func (m *Moto) Install(_ *harness.TestContext) error {
	if m.opts.Namespace == "" {
		return fmt.Errorf("moto namespace is required")
	}
	if err := util.Step("apply moto manifest", func() error {
		return harness.ApplyManifest(m.opts.Namespace, motoManifest)
	}); err != nil {
		return err
	}
	m.installed = true

	if err := util.Step("wait for moto pod ready", func() error {
		return harness.WaitForReady(m.opts.Namespace, motoSelector)
	}); err != nil {
		return err
	}

	return util.Step("wait for moto seed job", func() error {
		return harness.RunCommand("kubectl",
			"--namespace", m.opts.Namespace,
			"wait", "--for=condition=complete", "job/moto-seed",
			"--timeout=2m",
		)
	})
}

func (m *Moto) Cleanup() {
	if !m.installed {
		return
	}
	_ = harness.DeleteManifest(m.opts.Namespace, motoManifest)
}
