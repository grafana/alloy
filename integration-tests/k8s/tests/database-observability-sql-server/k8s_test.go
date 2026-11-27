package databaseobservabilitysqlserver

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
	"github.com/grafana/alloy/internal/component/database_observability/sql_server/collector"
)

//go:embed config/seed.yaml
var seedManifest string

const (
	sqlServerDeployment = "deploy/mssql"
	sqlServerSelector   = "app=mssql"
	seedJob             = "job/sql-server-dbo-seed"
	seedJobSelector     = "app=sql-server-dbo-seed"
	sqlServerSAPassword = "Alloy_Integration_Test_1"
)

type sqlServerSeed struct {
	namespace string
	installed bool
}

func (s *sqlServerSeed) Name() string { return "sql-server-dbo-seed" }

func (s *sqlServerSeed) Install(ctx *harness.TestContext) error {
	s.installed = true
	if ctx != nil {
		ctx.AddDiagnosticHook("sql_server and seed logs", func(c context.Context) error {
			return harness.RunDiagnosticCommands(c, [][]string{
				{"kubectl", "--namespace", s.namespace, "logs", "-l", sqlServerSelector, "--all-containers=true", "--tail", "200"},
				{"kubectl", "--namespace", s.namespace, "get", "jobs", "-o", "wide"},
				{"kubectl", "--namespace", s.namespace, "logs", "-l", seedJobSelector, "--all-containers=true", "--tail", "200"},
			})
		})
	}
	if err := util.Step("apply sql_server seed manifest", func() error {
		return harness.ApplyManifest(s.namespace, seedManifest)
	}); err != nil {
		s.Cleanup()
		return err
	}
	if err := util.Step("wait for sql_server seed job", func() error {
		return harness.RunCommand(
			"kubectl",
			"--namespace",
			s.namespace,
			"wait",
			"--for=condition=complete",
			seedJob,
			"--timeout=5m",
		)
	}); err != nil {
		s.Cleanup()
		return err
	}
	return nil
}

func (s *sqlServerSeed) Cleanup() {
	if !s.installed {
		return
	}
	_ = harness.DeleteManifest(s.namespace, seedManifest)
}

func querySQLServer(namespace, database, query string) (string, error) {
	script := `set -eu
SQLCMD=""
for candidate in /opt/mssql-tools18/bin/sqlcmd /opt/mssql-tools/bin/sqlcmd; do
  if [ -x "$candidate" ]; then
    SQLCMD="$candidate"
    break
  fi
done
if [ -z "$SQLCMD" ]; then
  echo "sqlcmd binary not found in container" >&2
  exit 1
fi
exec "$SQLCMD" -C -b -h -1 -W -S 127.0.0.1 -U sa -P "$1" -d "$2" -Q "$3"
`
	out, err := harness.RunCommandOutput(
		"kubectl",
		"--namespace",
		namespace,
		"exec",
		sqlServerDeployment,
		"--",
		"sh",
		"-c",
		script,
		"sqlcmd-exec",
		sqlServerSAPassword,
		database,
		query,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func TestDatabaseObservabilitySQLServer(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-database-observability-sql-server",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	sqlServer := deps.NewMSSQL(deps.MSSQLOptions{Namespace: ns.Name()})
	seed := &sqlServerSeed{namespace: ns.Name()}
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-dbo-sql-server",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, sqlServer, seed, mimir, loki, alloy},
	})

	const testName = "database-observability-sql-server"

	metrics := []string{
		"database_observability_connection_info",
		"database_observability_query_executions_total",
		"database_observability_query_duration_seconds_total",
		"database_observability_query_errors_total",
	}
	mimir.QueryMetrics(t, testName, metrics)
	mimir.QueryPositive(t, testName, []string{"database_observability_connection_info"})

	loki.QueryLogsPresent(t, testName,
		deps.LogMatcher{Labels: map[string]string{"op": "health_status"}},
		deps.LogMatcher{Labels: map[string]string{"op": "query_association"}},
		deps.LogMatcher{Labels: map[string]string{"op": "query_parsed_table_names_v2"}},
		deps.LogMatcher{Labels: map[string]string{"op": "table_detection"}},
		deps.LogMatcher{Labels: map[string]string{"op": "create_statement"}},
		deps.LogMatcher{Labels: map[string]string{"op": "explain_plan_output"}},
	)

	markerQuery := fmt.Sprintf(
		"SET NOCOUNT ON; EXEC sys.sp_query_store_flush_db; SELECT COUNT(*) FROM sys.query_store_query_text WHERE CHARINDEX(N'%s', query_sql_text) > 0;",
		collector.QueryMarker,
	)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		out, err := querySQLServer(ns.Name(), "testdb", markerQuery)
		require.NoError(c, err)
		count, err := strconv.Atoi(out)
		require.NoErrorf(c, err, "unexpected sqlcmd output %q", out)
		require.Greater(c, count, 0, "expected SQL Server Query Store to preserve %s in query_sql_text", collector.QueryMarker)
	}, 1*time.Minute, 500*time.Millisecond)
}
