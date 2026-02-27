package mysql

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfigUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
	data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
	enable_collectors = ["collector1"]
	disable_collectors = ["collector2"]
	set_collectors = ["collector3", "collector4"]
	lock_wait_timeout = 1
	log_slow_filter = false

	info_schema.processlist {
		min_time = 2
		processes_by_user = true
		processes_by_host = false
	}

	info_schema.tables {
		databases = "schema"
	}

	perf_schema.eventsstatements {
		limit = 3
		time_limit = 4
		text_limit = 5
		drop_digest_text = true
	}

	perf_schema.file_instances {
		filter = "instances_filter"
		remove_prefix = "instances_remove"
	}

	perf_schema.memory_events {
		remove_prefix = "innodb/"
	}

	heartbeat {
		database = "heartbeat_database"
		table = "heartbeat_table"
		utc = true
	}

	mysql.user {
		privileges = false
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	require.Equal(t, "root:secret_password@tcp(localhost:3306)/mydb", string(args.DataSourceName))
	require.Equal(t, []string{"collector1"}, args.EnableCollectors)
	require.Equal(t, []string{"collector2"}, args.DisableCollectors)
	require.Equal(t, []string{"collector3", "collector4"}, args.SetCollectors)
	require.Equal(t, 1, args.LockWaitTimeout)
	require.False(t, args.LogSlowFilter)
	require.Equal(t, 2, args.InfoSchemaProcessList.MinTime)
	require.True(t, args.InfoSchemaProcessList.ProcessesByUser)
	require.False(t, args.InfoSchemaProcessList.ProcessesByHost)
	require.Equal(t, "schema", args.InfoSchemaTables.Databases)
	require.Equal(t, 3, args.PerfSchemaEventsStatements.Limit)
	require.Equal(t, 4, args.PerfSchemaEventsStatements.TimeLimit)
	require.Equal(t, 5, args.PerfSchemaEventsStatements.TextLimit)
	require.True(t, args.PerfSchemaEventsStatements.DropDigestText)
	require.Equal(t, "instances_filter", args.PerfSchemaFileInstances.Filter)
	require.Equal(t, "instances_remove", args.PerfSchemaFileInstances.RemovePrefix)
	require.Equal(t, "innodb/", args.PerfSchemaMemoryEvents.RemovePrefix)
	require.Equal(t, "heartbeat_database", args.Heartbeat.Database)
	require.Equal(t, "heartbeat_table", args.Heartbeat.Table)
	require.True(t, args.Heartbeat.UTC)
	require.False(t, args.MySQLUser.Privileges)
}

func TestAlloyConfigConvert(t *testing.T) {
	var exampleAlloyConfig = `
	data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
	enable_collectors = ["collector1"]
	disable_collectors = ["collector2"]
	set_collectors = ["collector3", "collector4"]
	lock_wait_timeout = 1
	log_slow_filter = false
	
	info_schema.processlist {
		min_time = 2
		processes_by_user = true
		processes_by_host = false
	}

	info_schema.tables {
		databases = "schema"
	}

	perf_schema.eventsstatements {
		limit = 3
		time_limit = 4
		text_limit = 5
	}

	perf_schema.file_instances {
		filter = "instances_filter"
		remove_prefix = "instances_remove"
	}

	perf_schema.memory_events {
		remove_prefix = "innodb/"
	}

	heartbeat {
		database = "heartbeat_database"
		table = "heartbeat_table"
		utc = true
	}

	mysql.user {
		privileges = false
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	c := args.Convert()
	require.Equal(t, "root:secret_password@tcp(localhost:3306)/mydb", string(c.DataSourceName))
	require.Equal(t, []string{"collector1"}, c.EnableCollectors)
	require.Equal(t, []string{"collector2"}, c.DisableCollectors)
	require.Equal(t, []string{"collector3", "collector4"}, c.SetCollectors)
	require.Equal(t, 1, c.LockWaitTimeout)
	require.False(t, c.LogSlowFilter)
	require.Equal(t, 2, c.InfoSchemaProcessListMinTime)
	require.True(t, c.InfoSchemaProcessListProcessesByUser)
	require.False(t, c.InfoSchemaProcessListProcessesByHost)
	require.Equal(t, "schema", c.InfoSchemaTablesDatabases)
	require.Equal(t, 3, c.PerfSchemaEventsStatementsLimit)
	require.Equal(t, 4, c.PerfSchemaEventsStatementsTimeLimit)
	require.Equal(t, 5, c.PerfSchemaEventsStatementsTextLimit)
	require.Equal(t, "instances_filter", c.PerfSchemaFileInstancesFilter)
	require.Equal(t, "instances_remove", c.PerfSchemaFileInstancesRemovePrefix)
	require.Equal(t, "innodb/", c.PerfSchemaMemoryEventsRemovePrefix)
	require.Equal(t, "heartbeat_database", c.HeartbeatDatabase)
	require.Equal(t, "heartbeat_table", c.HeartbeatTable)
	require.True(t, c.HeartbeatUTC)
	require.False(t, c.MySQLUserPrivileges)
}

// Checks that the configs have not drifted between Grafana Agent static mode and Alloy.
func TestDefaultsSame(t *testing.T) {
	convertedDefaults := DefaultArguments.Convert()
	require.Equal(t, mysqld_exporter.DefaultConfig, *convertedDefaults)
}

func TestValidate_ValidDataSource(t *testing.T) {
	args := Arguments{
		DataSourceName: alloytypes.Secret("root:secret_password@tcp(localhost:3306)/mydb"),
	}
	require.NoError(t, args.Validate())
}

func TestValidate_InvalidDataSource(t *testing.T) {
	args := Arguments{
		DataSourceName: alloytypes.Secret("root:secret_password@invalid/mydb"),
	}
	require.Error(t, args.Validate())
}

func TestLabelDropHandler_DropsSpecifiedLabel(t *testing.T) {
	const metricsInput = `# HELP mysql_perf_schema_events_statements_total Total events statements.
# TYPE mysql_perf_schema_events_statements_total counter
mysql_perf_schema_events_statements_total{digest="abc123",digest_text="SELECT * FROM foo",schema="mydb"} 42
mysql_perf_schema_events_statements_total{digest="def456",digest_text="INSERT INTO bar VALUES (?)",schema="mydb"} 7
# HELP mysql_up Whether MySQL is up.
# TYPE mysql_up gauge
mysql_up{instance="localhost:3306"} 1
`

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metricsInput))
	})

	handler := newLabelDropHandler(inner, []string{"digest_text"})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	output := string(body)

	// digest_text must be gone
	require.NotContains(t, output, "digest_text")

	// other labels and values must be preserved
	require.Contains(t, output, `digest="abc123"`)
	require.Contains(t, output, `digest="def456"`)
	require.Contains(t, output, `schema="mydb"`)
	require.Contains(t, output, "42")
	require.Contains(t, output, "7")

	// unrelated metrics must be untouched
	require.Contains(t, output, `mysql_up`)
	require.Contains(t, output, `instance="localhost:3306"`)
}

func TestLabelDropHandler_PreservesMetricsWhenLabelAbsent(t *testing.T) {
	const metricsInput = `# HELP mysql_up Whether MySQL is up.
# TYPE mysql_up gauge
mysql_up{instance="localhost:3306"} 1
`

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metricsInput))
	})

	handler := newLabelDropHandler(inner, []string{"digest_text"})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	output := string(body)

	require.Contains(t, output, `mysql_up`)
	require.Contains(t, output, `instance="localhost:3306"`)
	require.Contains(t, output, "1")
}

func TestDropDigestText_DefaultIsFalse(t *testing.T) {
	var args Arguments
	args.SetToDefault()
	require.False(t, args.PerfSchemaEventsStatements.DropDigestText)
}
