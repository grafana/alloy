package collector

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

var serverConfigurationColumns = []string{"name", "configured_value", "in_use_value"}

const testServerConfigurationQueryTimeout = time.Second

func newTestServerConfiguration(t *testing.T, engineEdition int) (*ServerConfiguration, sqlmock.Sqlmock, *prometheus.Registry) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	registry := prometheus.NewRegistry()
	c, err := NewServerConfiguration(ServerConfigurationArguments{
		DB:              db,
		Registry:        registry,
		CollectInterval: time.Hour,
		QueryTimeout:    testServerConfigurationQueryTimeout,
		Logger:          util.TestAlloyLogger(t).Slog(),
		EngineEdition:   engineEdition,
	})
	require.NoError(t, err)
	return c, mock, registry
}

func TestServerConfigurationCollectsAllowlistedSettings(t *testing.T) {
	c, mock, _ := newTestServerConfiguration(t, 3)

	mock.ExpectQuery(selectServerConfiguration).WillReturnRows(
		sqlmock.NewRows(serverConfigurationColumns).
			AddRow("user connections", 0, 0).
			AddRow("remote admin connections", 1, 1).
			AddRow("network packet size (B)", 4096, 8192).
			AddRow("remote login timeout (s)", 10, 10).
			AddRow("max server memory (MB)", 4096, 2048).
			AddRow("min server memory (MB)", 1024, 1024).
			AddRow("max worker threads", 0, 0).
			AddRow("max degree of parallelism", 8, 4).
			AddRow("cost threshold for parallelism", 50, 5).
			AddRow("optimize for ad hoc workloads", 1, 0).
			AddRow("backup compression default", 1, 1).
			AddRow("min memory per query (KB)", 1024, 2048).
			AddRow("index create memory (KB)", 0, 0),
	)
	require.NoError(t, c.collectWithTimeout(t.Context()))

	expected := `
# HELP database_observability_sql_server_config_backup_compression_default Configured and running values of the SQL Server backup compression default setting.
# TYPE database_observability_sql_server_config_backup_compression_default gauge
database_observability_sql_server_config_backup_compression_default{state="configured"} 1
database_observability_sql_server_config_backup_compression_default{state="in_use"} 1
# HELP database_observability_sql_server_config_cost_threshold_for_parallelism Configured and running values of the SQL Server cost threshold for parallelism setting.
# TYPE database_observability_sql_server_config_cost_threshold_for_parallelism gauge
database_observability_sql_server_config_cost_threshold_for_parallelism{state="configured"} 50
database_observability_sql_server_config_cost_threshold_for_parallelism{state="in_use"} 5
# HELP database_observability_sql_server_config_index_create_memory_bytes Configured and running minimum memory allocated for index creation, in bytes.
# TYPE database_observability_sql_server_config_index_create_memory_bytes gauge
database_observability_sql_server_config_index_create_memory_bytes{state="configured"} 0
database_observability_sql_server_config_index_create_memory_bytes{state="in_use"} 0
# HELP database_observability_sql_server_config_max_degree_of_parallelism Configured and running values of the SQL Server maximum degree of parallelism setting.
# TYPE database_observability_sql_server_config_max_degree_of_parallelism gauge
database_observability_sql_server_config_max_degree_of_parallelism{state="configured"} 8
database_observability_sql_server_config_max_degree_of_parallelism{state="in_use"} 4
# HELP database_observability_sql_server_config_max_server_memory_bytes Configured and running maximum memory available to the SQL Server memory manager, in bytes.
# TYPE database_observability_sql_server_config_max_server_memory_bytes gauge
database_observability_sql_server_config_max_server_memory_bytes{state="configured"} 4.294967296e+09
database_observability_sql_server_config_max_server_memory_bytes{state="in_use"} 2.147483648e+09
# HELP database_observability_sql_server_config_max_worker_threads Configured and running values of the SQL Server maximum worker threads setting.
# TYPE database_observability_sql_server_config_max_worker_threads gauge
database_observability_sql_server_config_max_worker_threads{state="configured"} 0
database_observability_sql_server_config_max_worker_threads{state="in_use"} 0
# HELP database_observability_sql_server_config_min_memory_per_query_bytes Configured and running minimum memory allocated for query execution, in bytes.
# TYPE database_observability_sql_server_config_min_memory_per_query_bytes gauge
database_observability_sql_server_config_min_memory_per_query_bytes{state="configured"} 1.048576e+06
database_observability_sql_server_config_min_memory_per_query_bytes{state="in_use"} 2.097152e+06
# HELP database_observability_sql_server_config_min_server_memory_bytes Configured and running minimum memory retained by the SQL Server memory manager, in bytes.
# TYPE database_observability_sql_server_config_min_server_memory_bytes gauge
database_observability_sql_server_config_min_server_memory_bytes{state="configured"} 1.073741824e+09
database_observability_sql_server_config_min_server_memory_bytes{state="in_use"} 1.073741824e+09
# HELP database_observability_sql_server_config_network_packet_size_bytes Configured and running SQL Server network packet size, in bytes.
# TYPE database_observability_sql_server_config_network_packet_size_bytes gauge
database_observability_sql_server_config_network_packet_size_bytes{state="configured"} 4096
database_observability_sql_server_config_network_packet_size_bytes{state="in_use"} 8192
# HELP database_observability_sql_server_config_optimize_for_ad_hoc_workloads Configured and running values of the SQL Server optimize for ad hoc workloads setting.
# TYPE database_observability_sql_server_config_optimize_for_ad_hoc_workloads gauge
database_observability_sql_server_config_optimize_for_ad_hoc_workloads{state="configured"} 1
database_observability_sql_server_config_optimize_for_ad_hoc_workloads{state="in_use"} 0
# HELP database_observability_sql_server_config_remote_admin_connections Configured and running values of the SQL Server remote admin connections setting.
# TYPE database_observability_sql_server_config_remote_admin_connections gauge
database_observability_sql_server_config_remote_admin_connections{state="configured"} 1
database_observability_sql_server_config_remote_admin_connections{state="in_use"} 1
# HELP database_observability_sql_server_config_remote_login_timeout_seconds Configured and running SQL Server timeout for remote login attempts, in seconds.
# TYPE database_observability_sql_server_config_remote_login_timeout_seconds gauge
database_observability_sql_server_config_remote_login_timeout_seconds{state="configured"} 10
database_observability_sql_server_config_remote_login_timeout_seconds{state="in_use"} 10
# HELP database_observability_sql_server_config_user_connections Configured and running values of the SQL Server user connections setting.
# TYPE database_observability_sql_server_config_user_connections gauge
database_observability_sql_server_config_user_connections{state="configured"} 0
database_observability_sql_server_config_user_connections{state="in_use"} 0
`
	require.NoError(t, testutil.CollectAndCompare(c, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerConfigurationToleratesProviderSubsets(t *testing.T) {
	c, mock, _ := newTestServerConfiguration(t, azureSQLManagedInstanceEngineEdition)

	mock.ExpectQuery(selectServerConfiguration).WillReturnRows(
		sqlmock.NewRows(serverConfigurationColumns).
			AddRow("max server memory (MB)", 8192, 8192).
			AddRow("max worker threads", 512, 512).
			AddRow("max degree of parallelism", 8, 8),
	)
	require.NoError(t, c.collectWithTimeout(t.Context()))

	require.NoError(t, testutil.CollectAndCompare(
		c,
		strings.NewReader(`
# HELP database_observability_sql_server_config_max_worker_threads Configured and running values of the SQL Server maximum worker threads setting.
# TYPE database_observability_sql_server_config_max_worker_threads gauge
database_observability_sql_server_config_max_worker_threads{state="configured"} 512
database_observability_sql_server_config_max_worker_threads{state="in_use"} 512
`),
		"database_observability_sql_server_config_max_worker_threads",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerConfigurationAzureSQLDatabaseCollectsUsefulSettings(t *testing.T) {
	c, mock, _ := newTestServerConfiguration(t, azureSQLDatabaseEngineEdition)

	mock.ExpectQuery(selectAzureSQLDatabaseServerConfiguration).WillReturnRows(
		sqlmock.NewRows(serverConfigurationColumns).
			AddRow("network packet size (B)", 4096, 4096).
			AddRow("remote login timeout (s)", 10, 10).
			AddRow("min memory per query (KB)", 1024, 1024).
			AddRow("index create memory (KB)", 0, 0),
	)
	mock.ExpectQuery(selectDatabaseMaxDegreeOfParallelism).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(8))
	require.NoError(t, c.collectWithTimeout(t.Context()))

	expected := `
# HELP database_observability_sql_server_config_database_max_degree_of_parallelism Maximum degree of parallelism configured for the current Azure SQL Database.
# TYPE database_observability_sql_server_config_database_max_degree_of_parallelism gauge
database_observability_sql_server_config_database_max_degree_of_parallelism 8
# HELP database_observability_sql_server_config_index_create_memory_bytes Configured and running minimum memory allocated for index creation, in bytes.
# TYPE database_observability_sql_server_config_index_create_memory_bytes gauge
database_observability_sql_server_config_index_create_memory_bytes{state="configured"} 0
database_observability_sql_server_config_index_create_memory_bytes{state="in_use"} 0
# HELP database_observability_sql_server_config_min_memory_per_query_bytes Configured and running minimum memory allocated for query execution, in bytes.
# TYPE database_observability_sql_server_config_min_memory_per_query_bytes gauge
database_observability_sql_server_config_min_memory_per_query_bytes{state="configured"} 1.048576e+06
database_observability_sql_server_config_min_memory_per_query_bytes{state="in_use"} 1.048576e+06
# HELP database_observability_sql_server_config_network_packet_size_bytes Configured and running SQL Server network packet size, in bytes.
# TYPE database_observability_sql_server_config_network_packet_size_bytes gauge
database_observability_sql_server_config_network_packet_size_bytes{state="configured"} 4096
database_observability_sql_server_config_network_packet_size_bytes{state="in_use"} 4096
# HELP database_observability_sql_server_config_remote_login_timeout_seconds Configured and running SQL Server timeout for remote login attempts, in seconds.
# TYPE database_observability_sql_server_config_remote_login_timeout_seconds gauge
database_observability_sql_server_config_remote_login_timeout_seconds{state="configured"} 10
database_observability_sql_server_config_remote_login_timeout_seconds{state="in_use"} 10
`
	require.NoError(t, testutil.CollectAndCompare(c, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerConfigurationFailedRefreshRetainsLastSuccessfulValues(t *testing.T) {
	c, mock, _ := newTestServerConfiguration(t, 3)

	mock.ExpectQuery(selectServerConfiguration).WillReturnRows(
		sqlmock.NewRows(serverConfigurationColumns).AddRow("user connections", 100, 80),
	)
	require.NoError(t, c.collectWithTimeout(t.Context()))

	mock.ExpectQuery(selectServerConfiguration).WillReturnError(errors.New("configuration unavailable"))
	require.ErrorContains(t, c.collectWithTimeout(t.Context()), "configuration unavailable")

	require.NoError(t, testutil.CollectAndCompare(
		c,
		strings.NewReader(`
# HELP database_observability_sql_server_config_user_connections Configured and running values of the SQL Server user connections setting.
# TYPE database_observability_sql_server_config_user_connections gauge
database_observability_sql_server_config_user_connections{state="configured"} 100
database_observability_sql_server_config_user_connections{state="in_use"} 80
`),
		"database_observability_sql_server_config_user_connections",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerConfigurationAppliesQueryTimeout(t *testing.T) {
	assertTimesOut := func(t *testing.T, c *ServerConfiguration, mock sqlmock.Sqlmock) {
		t.Helper()
		startedAt := time.Now()
		err := c.collectWithTimeout(t.Context())
		require.Error(t, err)
		assert.Equal(t, testServerConfigurationQueryTimeout, time.Since(startedAt))
		require.NoError(t, mock.ExpectationsWereMet())
	}

	t.Run("server configuration", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, mock, _ := newTestServerConfiguration(t, 3)
			mock.ExpectQuery(selectServerConfiguration).
				WillDelayFor(time.Hour).
				WillReturnRows(sqlmock.NewRows(serverConfigurationColumns))
			assertTimesOut(t, c, mock)
		})
	})

	t.Run("Azure SQL Database server configuration", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, mock, _ := newTestServerConfiguration(t, azureSQLDatabaseEngineEdition)
			mock.ExpectQuery(selectAzureSQLDatabaseServerConfiguration).
				WillDelayFor(time.Hour).
				WillReturnRows(sqlmock.NewRows(serverConfigurationColumns))
			assertTimesOut(t, c, mock)
		})
	})

	t.Run("Azure SQL Database scoped configuration", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c, mock, _ := newTestServerConfiguration(t, azureSQLDatabaseEngineEdition)
			mock.ExpectQuery(selectAzureSQLDatabaseServerConfiguration).
				WillReturnRows(sqlmock.NewRows(serverConfigurationColumns))
			mock.ExpectQuery(selectDatabaseMaxDegreeOfParallelism).
				WillDelayFor(time.Hour).
				WillReturnRows(sqlmock.NewRows([]string{"value"}))
			assertTimesOut(t, c, mock)
		})
	})
}

func TestServerConfigurationStartAndStop(t *testing.T) {
	c, mock, registry := newTestServerConfiguration(t, 3)
	mock.ExpectQuery(selectServerConfiguration).WillReturnRows(
		sqlmock.NewRows(serverConfigurationColumns).AddRow("user connections", 0, 0),
	)

	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()
	require.Eventually(t, func() bool {
		c.metricsMut.RLock()
		defer c.metricsMut.RUnlock()
		return len(c.metrics) == 2
	}, 5*time.Second, 20*time.Millisecond)
	assert.False(t, c.Stopped())
	count, err := testutil.GatherAndCount(
		registry,
		"database_observability_sql_server_config_user_connections",
	)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	c.Stop()
	assert.True(t, c.Stopped())
	count, err = testutil.GatherAndCount(
		registry,
		"database_observability_sql_server_config_user_connections",
	)
	require.NoError(t, err)
	assert.Zero(t, count)
	require.NoError(t, mock.ExpectationsWereMet())
}
