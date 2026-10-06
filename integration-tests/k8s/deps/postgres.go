package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/postgres.yaml
var postgresManifest string

const postgresSelector = "app=postgres"

var _ harness.Dependency = (*Postgres)(nil)

// Postgres installs a Postgres instance with pg_stat_statements preloaded and a
// monitoring_user role, seeded with tables, rows and a few queries so
// database_observability.postgres has data to report.
type Postgres struct {
	opts      PostgresOptions
	installed bool
}

type PostgresOptions struct {
	Namespace string
}

func NewPostgres(opts PostgresOptions) *Postgres {
	return &Postgres{opts: opts}
}

func (m *Postgres) Name() string { return "postgres" }

func (m *Postgres) Install(_ *harness.TestContext) error {
	if m.opts.Namespace == "" {
		return fmt.Errorf("postgres namespace is required")
	}
	// Marked before applying so a partially applied manifest is still cleaned up.
	m.installed = true

	if err := util.Step("apply postgres manifest", func() error {
		return harness.ApplyManifest(m.opts.Namespace, postgresManifest)
	}); err != nil {
		m.Cleanup()
		return err
	}

	if err := util.Step("wait for postgres pod ready", func() error {
		return harness.WaitForReady(m.opts.Namespace, postgresSelector)
	}); err != nil {
		m.Cleanup()
		return err
	}

	return nil
}

func (m *Postgres) Cleanup() {
	if !m.installed {
		return
	}
	_ = harness.DeleteManifest(m.opts.Namespace, postgresManifest)
}
