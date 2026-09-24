package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/mysql.yaml
var mysqlManifest string

const mysqlSelector = "app=mysql"

var _ harness.Dependency = (*MySQL)(nil)

// MySQL installs a MySQL instance seeded with tables, rows and a few queries so
// performance_schema has data for the database_observability.mysql component.
type MySQL struct {
	opts      MySQLOptions
	installed bool
}

type MySQLOptions struct {
	Namespace string
}

func NewMySQL(opts MySQLOptions) *MySQL {
	return &MySQL{opts: opts}
}

func (m *MySQL) Name() string { return "mysql" }

func (m *MySQL) Install(_ *harness.TestContext) error {
	if m.opts.Namespace == "" {
		return fmt.Errorf("mysql namespace is required")
	}
	// Marked before applying so a partially applied manifest is still cleaned up.
	m.installed = true

	if err := util.Step("apply mysql manifest", func() error {
		return harness.ApplyManifest(m.opts.Namespace, mysqlManifest)
	}); err != nil {
		m.Cleanup()
		return err
	}

	if err := util.Step("wait for mysql pod ready", func() error {
		return harness.WaitForReady(m.opts.Namespace, mysqlSelector)
	}); err != nil {
		m.Cleanup()
		return err
	}

	return nil
}

func (m *MySQL) Cleanup() {
	if !m.installed {
		return
	}
	_ = harness.DeleteManifest(m.opts.Namespace, mysqlManifest)
}
