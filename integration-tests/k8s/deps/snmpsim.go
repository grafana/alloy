package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/snmpsim.yaml
var snmpSimManifest string

const (
	snmpSimImage    = "tandrup/snmpsim:v0.4"
	snmpSimSelector = "app=snmpsim"
)

var _ harness.Dependency = (*SNMPSim)(nil)

// SNMPSim installs an SNMP agent simulator as a target for the
// prometheus.exporter.snmp component. It answers SNMP v2c on UDP 161 with the
// community "public".
type SNMPSim struct {
	opts      SNMPSimOptions
	installed bool
}

type SNMPSimOptions struct {
	Namespace string
}

func NewSNMPSim(opts SNMPSimOptions) *SNMPSim {
	return &SNMPSim{opts: opts}
}

func (s *SNMPSim) Name() string { return "snmpsim" }

func (s *SNMPSim) Install(_ *harness.TestContext) error {
	if s.opts.Namespace == "" {
		return fmt.Errorf("snmpsim namespace is required")
	}
	if err := ensureKindImage(snmpSimImage); err != nil {
		return err
	}
	if err := util.Step("apply snmpsim manifest", func() error {
		return harness.ApplyManifest(s.opts.Namespace, snmpSimManifest)
	}); err != nil {
		return err
	}
	s.installed = true

	return util.Step("wait for snmpsim pod ready", func() error {
		return harness.WaitForReady(s.opts.Namespace, snmpSimSelector)
	})
}

func (s *SNMPSim) Cleanup() {
	if !s.installed {
		return
	}
	_ = harness.DeleteManifest(s.opts.Namespace, snmpSimManifest)
}
