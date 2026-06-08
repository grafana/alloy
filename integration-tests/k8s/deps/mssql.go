package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/mssql.yaml
var mssqlManifest string

const (
	mssqlImage    = "mcr.microsoft.com/mssql/server:2022-latest"
	mssqlSelector = "app=mssql"
)

// MSSQL installs a Microsoft SQL Server instance as a scrape target for the
// prometheus.exporter.mssql component. The image is linux/amd64 only and runs
// under emulation on arm64 hosts.
type MSSQL struct {
	opts      MSSQLOptions
	installed bool
}

type MSSQLOptions struct {
	Namespace string
}

func NewMSSQL(opts MSSQLOptions) *MSSQL {
	return &MSSQL{opts: opts}
}

func (m *MSSQL) Name() string { return "mssql" }

func (m *MSSQL) Install(_ *harness.TestContext) error {
	if m.opts.Namespace == "" {
		return fmt.Errorf("mssql namespace is required")
	}
	if err := ensureKindImage(mssqlImage); err != nil {
		return err
	}
	if err := util.Step("apply mssql manifest", func() error {
		return harness.ApplyManifest(m.opts.Namespace, mssqlManifest)
	}); err != nil {
		return err
	}
	m.installed = true
	return util.Step("wait for mssql pod ready", func() error {
		return harness.WaitForReady(m.opts.Namespace, mssqlSelector)
	})
}

func (m *MSSQL) Cleanup() {
	if !m.installed {
		return
	}
	_ = harness.DeleteManifest(m.opts.Namespace, mssqlManifest)
}
