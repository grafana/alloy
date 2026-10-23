package dbinstance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/loki/pkg/push"
	pgcollector "github.com/prometheus-community/postgres_exporter/collector"
	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const testDSN = "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"

// testConfig runs only the always-on collectors, against an external exporter.
func testConfig() Config {
	return Config{
		DSN: testDSN,
		Collectors: map[string]bool{
			collector.QueryDetailsCollector:  false,
			collector.QuerySamplesCollector:  false,
			collector.SchemaDetailsCollector: false,
			collector.ExplainPlanCollector:   false,
			collector.TableStatsCollector:    false,
			collector.IndexStatsCollector:    false,
		},
		HealthCheck: HealthCheckArguments{CollectInterval: time.Hour},
	}
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func expectServerInfo(mock sqlmock.Sqlmock) {
	mock.ExpectPing()
	mock.ExpectQuery(selectServerInfo).
		WillReturnRows(sqlmock.NewRows([]string{"system_identifier", "inet_server_addr", "inet_server_port", "version"}).
			AddRow("1234567890", "127.0.0.1", "5432", "14.0"))
}

// newEntries is never stopped: the label middleware's goroutine outlives
// Instance.Stop and may still be sending to it.
func newEntries() *loki.CollectingHandler {
	return loki.NewCollectingHandler()
}

func newTestInstance(t *testing.T, cfg Config, entries loki.EntryHandler, dbs ...*sql.DB) *Instance {
	t.Helper()
	opened := 0
	inst, err := New(cfg, Deps{
		Logger:  logging.NewSlogNop(),
		Entries: entries,
		OpenDB: func(_, _ string) (*sql.DB, error) {
			db := dbs[opened]
			opened++
			return db, nil
		},
	})
	require.NoError(t, err)
	return inst
}

func TestServerIDComputedOnceOnConnect(t *testing.T) {
	db1, mock1 := newMockDB(t)
	db2, mock2 := newMockDB(t)
	expectServerInfo(mock1)
	mock1.ExpectClose()
	expectServerInfo(mock2)

	entries := newEntries()
	inst := newTestInstance(t, testConfig(), entries, db1, db2)
	defer inst.Stop()

	want := fmt.Sprintf("%x", sha256.Sum256([]byte("1234567890:127.0.0.1:5432")))

	require.NoError(t, inst.Connect(context.Background()))
	assert.Equal(t, want, inst.ServerID())

	// The second Connect closes the first connection and queries server info
	// on the new one; a second query on either mock would fail as unexpected.
	require.NoError(t, inst.Connect(context.Background()))
	assert.Equal(t, want, inst.ServerID())
	require.NoError(t, mock1.ExpectationsWereMet())
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestEntriesCarryServerID(t *testing.T) {
	db, mock := newMockDB(t)
	expectServerInfo(mock)

	entries := newEntries()
	inst := newTestInstance(t, testConfig(), entries, db)
	defer inst.Stop()

	require.NoError(t, inst.Connect(context.Background()))
	require.NotEmpty(t, inst.ServerID())

	// The health_check collector always reports the Alloy version on start.
	require.Eventually(t, func() bool {
		for _, e := range entries.Received() {
			if e.Labels["op"] == database_observability.OP_HEALTH_STATUS {
				return true
			}
		}
		return false
	}, 5*time.Second, 50*time.Millisecond)

	for _, e := range entries.Received() {
		assert.Equal(t, model.LabelValue(inst.ServerID()), e.Labels["server_id"])
		assert.Equal(t, model.LabelValue(inst.InstanceKey()), e.Labels["instance"])
	}
}

func TestStopReleasesConnection(t *testing.T) {
	db, mock := newMockDB(t)
	expectServerInfo(mock)
	mock.ExpectClose()

	entries := newEntries()
	inst := newTestInstance(t, testConfig(), entries, db)

	require.NoError(t, inst.Connect(context.Background()))
	require.True(t, inst.Started())

	inst.Stop()

	assert.False(t, inst.Started())
	assert.Nil(t, inst.db)
	require.NoError(t, mock.ExpectationsWereMet())
}

// forbiddenDep reports whether an Alloy package is part of the Alloy runtime.
func forbiddenDep(pkg string) bool {
	const alloy = "github.com/grafana/alloy/"
	if !strings.HasPrefix(pkg, alloy) {
		return false
	}
	p := strings.TrimPrefix(pkg, alloy)
	switch {
	case p == "internal/component":
		return true
	case strings.HasPrefix(p, "internal/component/"):
		rest := strings.TrimPrefix(p, "internal/component/")
		return rest != "common/loki" && rest != "common/relabel" &&
			rest != "database_observability" && !strings.HasPrefix(rest, "database_observability/")
	case strings.HasPrefix(p, "internal/service/"), strings.HasPrefix(p, "internal/runtime/"):
		return true
	case p == "syntax" || strings.HasPrefix(p, "syntax/"):
		return true
	}
	return false
}

// POC-HACK: the collectors import internal/runtime/logging for its Level
// type, which pulls in eventlog and the syntax tree (alloytypes included).
// Those edges are allowed; nothing else may reach a forbidden package.
func hackedDep(pkg string) bool {
	const alloy = "github.com/grafana/alloy/"
	p := strings.TrimPrefix(pkg, alloy)
	return p == "internal/runtime/logging" || strings.HasPrefix(p, "internal/runtime/logging/") ||
		p == "syntax" || strings.HasPrefix(p, "syntax/")
}

func TestNewNeedsNoAlloyRuntime(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, ".")
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	var violations []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		pkg, imports := fields[0], fields[1:]
		if hackedDep(pkg) {
			continue
		}
		if forbiddenDep(pkg) {
			violations = append(violations, pkg)
			continue
		}
		for _, imp := range imports {
			if forbiddenDep(imp) && !hackedDep(imp) {
				violations = append(violations, pkg+" -> "+imp)
			}
		}
	}
	assert.Empty(t, violations)
}

func TestAddLokiLabels(t *testing.T) {
	t.Run("add required labels to loki entries", func(t *testing.T) {
		handler := loki.NewCollectingHandler()
		defer handler.Stop()
		entryHandler := addLokiLabels(handler, "some-instance-key", "some-system-id", nil)

		go func() {
			ts := time.Now().UnixNano()
			entryHandler.Chan() <- loki.Entry{
				Entry: push.Entry{
					Timestamp: time.Unix(0, ts),
					Line:      "some-message",
				},
			}
		}()

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		require.Len(t, handler.Received(), 1)
		assert.Equal(t, model.LabelSet{
			"job":       database_observability.JobName,
			"instance":  model.LabelValue("some-instance-key"),
			"server_id": model.LabelValue("some-system-id"),
			"engine":    model.LabelValue(collector.EngineName),
		}, handler.Received()[0].Labels)
		assert.Equal(t, "some-message", handler.Received()[0].Line)
	})

	t.Run("per-database rules run after the required labels", func(t *testing.T) {
		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		staticRule := relabel.DefaultRelabelConfig
		staticRule.TargetLabel = "environment"
		staticRule.Replacement = "production"

		// Derives a label from server_id, which addLokiLabels sets, to prove
		// the rules see the component's own labels.
		var shortIDRegex relabel.Regexp
		require.NoError(t, shortIDRegex.UnmarshalText([]byte("(.{4}).*")))

		derivedRule := relabel.DefaultRelabelConfig
		derivedRule.SourceLabels = []string{"server_id"}
		derivedRule.Regex = shortIDRegex
		derivedRule.TargetLabel = "short_id"
		derivedRule.Replacement = "$1"

		entryHandler := addLokiLabels(handler, "some-instance-key", "abcdefgh", []*relabel.Config{&staticRule, &derivedRule})

		go func() {
			entryHandler.Chan() <- loki.Entry{
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "some-message",
				},
			}
		}()

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		assert.Equal(t, model.LabelSet{
			"job":         database_observability.JobName,
			"instance":    model.LabelValue("some-instance-key"),
			"server_id":   model.LabelValue("abcdefgh"),
			"engine":      model.LabelValue(collector.EngineName),
			"environment": model.LabelValue("production"),
			"short_id":    model.LabelValue("abcd"),
		}, handler.Received()[0].Labels)
	})
}

// newTestExporterRuntime builds a runtime without connecting; connections open on the first scrape.
func newTestExporterRuntime(t *testing.T) *pgcollector.Runtime {
	t.Helper()
	cfg := pgconfig.NewConfigWithDefaults()
	cfg.DataSourceNames = []string{testDSN}
	validated, err := cfg.Validate()
	require.NoError(t, err)
	runtime, err := pgcollector.NewRuntime(validated, nil)
	require.NoError(t, err)
	return runtime
}

func TestCleanupExporterCollectors(t *testing.T) {
	t.Run("unregisters collectors without a runtime", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		col := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "test_cleanup_exporter_collectors_plain",
			Help: "test",
		})
		col.Set(1)
		require.NoError(t, registry.Register(col))

		inst := &Instance{
			registry:           registry,
			exporterCollectors: []prometheus.Collector{col},
		}

		inst.cleanupExporterCollectors()

		assert.Nil(t, inst.exporterCollectors)
		assert.False(t, registry.Unregister(col))
	})

	t.Run("closes the runtime", func(t *testing.T) {
		runtime := newTestExporterRuntime(t)

		inst := &Instance{registry: prometheus.NewRegistry(), exporterRuntime: runtime}
		for _, col := range runtime.Collectors() {
			require.NoError(t, inst.registry.Register(col))
			inst.exporterCollectors = append(inst.exporterCollectors, col)
		}

		inst.cleanupExporterCollectors()

		assert.Nil(t, inst.exporterRuntime)
		assert.Nil(t, inst.exporterCollectors)
		for _, col := range runtime.Collectors() {
			assert.False(t, inst.registry.Unregister(col))
		}
	})
}

func TestStopReleasesExporter(t *testing.T) {
	inst, err := New(testConfig(), Deps{Logger: logging.NewSlogNop()})
	require.NoError(t, err)

	oldCollector := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_stop_old_exporter", Help: "test"})
	require.NoError(t, inst.registry.Register(oldCollector))
	inst.exporterCollectors = []prometheus.Collector{oldCollector}
	inst.exporterRuntime = newTestExporterRuntime(t)

	inst.Stop()

	assert.Nil(t, inst.exporterRuntime)
	assert.Nil(t, inst.exporterCollectors)
	assert.False(t, inst.registry.Unregister(oldCollector))
}
