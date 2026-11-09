package collector

import (
	"database/sql"
	"fmt"
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

var serverMetricsColumns = []string{
	"process_user_cpu_seconds",
	"process_kernel_cpu_seconds",
	"uptime_seconds",
	"logical_cpu_count",
	"memory_manager_committed_bytes",
	"memory_manager_target_bytes",
	"host_physical_memory_total_bytes",
	"host_physical_memory_available_bytes",
	"host_commit_limit_bytes",
	"host_commit_available_bytes",
	"host_high_memory_signal",
	"host_low_memory_signal",
	"process_resident_memory_bytes",
	"process_virtual_memory_bytes",
	"process_available_commit_memory_bytes",
	"process_committed_memory_resident_ratio",
	"process_page_faults",
	"process_physical_memory_low",
	"process_virtual_memory_low",
}

var userSessionColumns = []string{
	"running_sessions",
	"sleeping_sessions",
	"dormant_sessions",
	"preconnect_sessions",
	"configured_session_limit",
}

var azureSQLDatabaseResourceMetricColumns = []string{
	"end_time",
	"database_cpu_utilization_ratio",
	"database_memory_utilization_ratio",
	"database_data_io_utilization_ratio",
	"database_log_write_utilization_ratio",
	"database_worker_utilization_ratio",
	"database_session_utilization_ratio",
	"instance_cpu_utilization_ratio",
	"instance_memory_utilization_ratio",
}

var azureSQLDatabaseUserSessionColumns = []string{
	"running_sessions",
	"sleeping_sessions",
	"dormant_sessions",
	"preconnect_sessions",
}

var azureSQLDatabaseLimitColumns = []string{
	"database_vcore_limit",
	"database_dtu_limit",
	"user_session_limit",
	"database_worker_limit",
	"database_resource_governor_max_degree_of_parallelism",
	"database_memory_grant_limit_ratio",
}

var managedInstanceResourceMetricColumns = []string{
	"start_time",
	"end_time",
	"instance_vcore_limit",
	"instance_cpu_utilization_ratio",
	"instance_storage_limit_bytes",
	"instance_storage_used_bytes",
	"io_requests",
	"io_bytes_read",
	"io_bytes_written",
}

var databaseStorageColumns = []string{
	"data_storage_used_bytes",
	"data_storage_allocated_bytes",
	"log_storage_used_bytes",
	"log_storage_allocated_bytes",
	"database_storage_limit_bytes",
}

var volumeStorageColumns = []string{
	"instance_storage_allocated_bytes",
	"instance_storage_limit_bytes",
	"instance_storage_used_bytes",
	"instance_storage_available_bytes",
}

var performanceCounterColumns = []string{
	"batch_requests_total",
	"transactions_total",
	"sql_compilations_total",
	"sql_recompilations_total",
	"page_life_expectancy_seconds",
	"buffer_cache_hit_count",
	"buffer_cache_hit_base",
	"checkpoint_pages_total",
	"lazy_writes_total",
	"free_list_stalls_total",
	"page_splits_total",
	"full_scans_total",
	"memory_grants_pending",
	"processes_blocked",
	"deadlocks_total",
	"lock_waits_total",
	"lock_wait_seconds_total",
	"logins_total",
	"connection_resets_total",
	"logouts_total",
}

var fileIOStatsColumns = []string{
	"io_read_bytes_total",
	"io_written_bytes_total",
	"io_reads_total",
	"io_writes_total",
	"io_read_stall_seconds_total",
	"io_write_stall_seconds_total",
}

var tempdbSpaceUsageColumns = []string{
	"user_objects_bytes",
	"internal_objects_bytes",
	"version_store_bytes",
	"mixed_extents_bytes",
	"unallocated_bytes",
}

func TestServerMetrics(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:                 db,
		Registry:           registry,
		Logger:             util.TestAlloyLogger(t).Slog(),
		EngineEdition:      3,
		ProductVersion:     "16.0.1000.6",
		ProductLevel:       "RTM",
		ProductBuild:       "1000",
		CompatibilityLevel: "160",
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectServerMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(serverMetricsColumns).
			AddRow(
				120.5,
				30.25,
				3600,
				8,
				1024,
				2048,
				8192,
				4096,
				16384,
				8192,
				1,
				0,
				3072,
				4096,
				1024,
				0.75,
				42,
				0,
				1,
			))
	mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(userSessionColumns).AddRow(2, 3, 0, 1, 0))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(databaseStorageColumns).AddRow(8192000, 16384000, 1024000, 2048000, nil))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(performanceCounterColumns).AddRow(
			5000, 1200, 400, 15, 1800, 990, 1000, 35, 12, 3, 45, 180, 2, 4, 1, 25, 1.5, 80, 350, 75,
		))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(fileIOStatsColumns).AddRow(1048576, 524288, 128, 64, 0.25, 0.5))
	mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(tempdbSpaceUsageColumns).AddRow(65536, 32768, 16384, 8192, 524288))
	mock.ExpectQuery(selectVolumeStorage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(volumeStorageColumns).AddRow(18432000, 107374182400, 53687091200, 53687091200))

	expected := `
# HELP database_observability_sql_server_batch_requests_total Total Transact-SQL command batches received by SQL Server.
# TYPE database_observability_sql_server_batch_requests_total counter
database_observability_sql_server_batch_requests_total 5000
# HELP database_observability_sql_server_buffer_cache_hit_ratio Ratio of pages found in the buffer pool without reading from disk.
# TYPE database_observability_sql_server_buffer_cache_hit_ratio gauge
database_observability_sql_server_buffer_cache_hit_ratio 0.99
# HELP database_observability_sql_server_checkpoint_pages_total Total pages flushed to disk by checkpoint operations.
# TYPE database_observability_sql_server_checkpoint_pages_total counter
database_observability_sql_server_checkpoint_pages_total 35
# HELP database_observability_sql_server_connection_resets_total Total logins started from the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_connection_resets_total counter
database_observability_sql_server_connection_resets_total 350
# HELP database_observability_sql_server_database_storage_allocated_bytes Space allocated to the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_allocated_bytes gauge
database_observability_sql_server_database_storage_allocated_bytes{file_type="log"} 2.048e+06
database_observability_sql_server_database_storage_allocated_bytes{file_type="rows"} 1.6384e+07
# HELP database_observability_sql_server_database_storage_used_bytes Space used within the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_used_bytes gauge
database_observability_sql_server_database_storage_used_bytes{file_type="log"} 1.024e+06
database_observability_sql_server_database_storage_used_bytes{file_type="rows"} 8.192e+06
# HELP database_observability_sql_server_deadlocks_total Total lock requests that resulted in a deadlock.
# TYPE database_observability_sql_server_deadlocks_total counter
database_observability_sql_server_deadlocks_total 1
# HELP database_observability_sql_server_free_list_stalls_total Total requests that had to wait for a free page in the buffer pool.
# TYPE database_observability_sql_server_free_list_stalls_total counter
database_observability_sql_server_free_list_stalls_total 3
# HELP database_observability_sql_server_full_scans_total Total unrestricted base-table or full-index scans performed by SQL Server.
# TYPE database_observability_sql_server_full_scans_total counter
database_observability_sql_server_full_scans_total 180
# HELP database_observability_sql_server_host_commit_available_bytes Available operating system commit visible to the SQL Server host, in bytes.
# TYPE database_observability_sql_server_host_commit_available_bytes gauge
database_observability_sql_server_host_commit_available_bytes 8192
# HELP database_observability_sql_server_host_commit_limit_bytes Operating system commit limit visible to the SQL Server host, in bytes.
# TYPE database_observability_sql_server_host_commit_limit_bytes gauge
database_observability_sql_server_host_commit_limit_bytes 16384
# HELP database_observability_sql_server_host_high_memory_signal Whether the host high-memory resource notification is set.
# TYPE database_observability_sql_server_host_high_memory_signal gauge
database_observability_sql_server_host_high_memory_signal 1
# HELP database_observability_sql_server_host_logical_cpu_count Number of logical CPUs visible to the SQL Server host.
# TYPE database_observability_sql_server_host_logical_cpu_count gauge
database_observability_sql_server_host_logical_cpu_count 8
# HELP database_observability_sql_server_host_low_memory_signal Whether the host low-memory resource notification is set.
# TYPE database_observability_sql_server_host_low_memory_signal gauge
database_observability_sql_server_host_low_memory_signal 0
# HELP database_observability_sql_server_host_physical_memory_available_bytes Available physical memory visible to the SQL Server host, in bytes.
# TYPE database_observability_sql_server_host_physical_memory_available_bytes gauge
database_observability_sql_server_host_physical_memory_available_bytes 4096
# HELP database_observability_sql_server_host_physical_memory_total_bytes Total physical memory visible to the SQL Server host, in bytes.
# TYPE database_observability_sql_server_host_physical_memory_total_bytes gauge
database_observability_sql_server_host_physical_memory_total_bytes 8192
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="160",edition_name="enterprise",engine_edition="3",product_build="1000",product_level="RTM",product_version="16.0.1000.6"} 1
# HELP database_observability_sql_server_instance_storage_allocated_bytes Total space allocated to database files on the SQL Server instance, in bytes.
# TYPE database_observability_sql_server_instance_storage_allocated_bytes gauge
database_observability_sql_server_instance_storage_allocated_bytes 1.8432e+07
# HELP database_observability_sql_server_instance_storage_available_bytes Available storage capacity on operating system volumes hosting SQL Server files, in bytes.
# TYPE database_observability_sql_server_instance_storage_available_bytes gauge
database_observability_sql_server_instance_storage_available_bytes 5.36870912e+10
# HELP database_observability_sql_server_instance_storage_limit_bytes Reserved storage capacity of the Azure SQL Managed Instance or total operating system volume capacity, in bytes.
# TYPE database_observability_sql_server_instance_storage_limit_bytes gauge
database_observability_sql_server_instance_storage_limit_bytes 1.073741824e+11
# HELP database_observability_sql_server_instance_storage_used_bytes Storage used by the Azure SQL Managed Instance or across operating system volumes hosting SQL Server files, in bytes.
# TYPE database_observability_sql_server_instance_storage_used_bytes gauge
database_observability_sql_server_instance_storage_used_bytes 5.36870912e+10
# HELP database_observability_sql_server_io_read_bytes_total Total bytes read from SQL Server database and log files.
# TYPE database_observability_sql_server_io_read_bytes_total counter
database_observability_sql_server_io_read_bytes_total 1.048576e+06
# HELP database_observability_sql_server_io_read_stall_seconds_total Total time spent waiting for read operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_read_stall_seconds_total counter
database_observability_sql_server_io_read_stall_seconds_total 0.25
# HELP database_observability_sql_server_io_reads_total Total physical read operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_reads_total counter
database_observability_sql_server_io_reads_total 128
# HELP database_observability_sql_server_io_write_stall_seconds_total Total time spent waiting for write operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_write_stall_seconds_total counter
database_observability_sql_server_io_write_stall_seconds_total 0.5
# HELP database_observability_sql_server_io_writes_total Total physical write operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_writes_total counter
database_observability_sql_server_io_writes_total 64
# HELP database_observability_sql_server_io_written_bytes_total Total bytes written to SQL Server database and log files.
# TYPE database_observability_sql_server_io_written_bytes_total counter
database_observability_sql_server_io_written_bytes_total 524288
# HELP database_observability_sql_server_lazy_writes_total Total pages written to disk by the lazy writer to free buffer pool pages.
# TYPE database_observability_sql_server_lazy_writes_total counter
database_observability_sql_server_lazy_writes_total 12
# HELP database_observability_sql_server_lock_wait_seconds_total Total time spent waiting for locks, in seconds.
# TYPE database_observability_sql_server_lock_wait_seconds_total counter
database_observability_sql_server_lock_wait_seconds_total 1.5
# HELP database_observability_sql_server_lock_waits_total Total lock requests that required the caller to wait.
# TYPE database_observability_sql_server_lock_waits_total counter
database_observability_sql_server_lock_waits_total 25
# HELP database_observability_sql_server_logins_total Total logins started outside the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_logins_total counter
database_observability_sql_server_logins_total 80
# HELP database_observability_sql_server_logouts_total Total logout operations since SQL Server last started.
# TYPE database_observability_sql_server_logouts_total counter
database_observability_sql_server_logouts_total 75
# HELP database_observability_sql_server_memory_grants_pending Current number of processes waiting for a workspace memory grant.
# TYPE database_observability_sql_server_memory_grants_pending gauge
database_observability_sql_server_memory_grants_pending 2
# HELP database_observability_sql_server_memory_manager_committed_bytes Memory committed by the SQL Server memory manager, in bytes.
# TYPE database_observability_sql_server_memory_manager_committed_bytes gauge
database_observability_sql_server_memory_manager_committed_bytes 1024
# HELP database_observability_sql_server_memory_manager_target_bytes Memory the SQL Server memory manager can consume, in bytes.
# TYPE database_observability_sql_server_memory_manager_target_bytes gauge
database_observability_sql_server_memory_manager_target_bytes 2048
# HELP database_observability_sql_server_page_life_expectancy_seconds Duration in seconds that a data page stays in the buffer pool without references.
# TYPE database_observability_sql_server_page_life_expectancy_seconds gauge
database_observability_sql_server_page_life_expectancy_seconds 1800
# HELP database_observability_sql_server_page_splits_total Total index page splits performed by SQL Server.
# TYPE database_observability_sql_server_page_splits_total counter
database_observability_sql_server_page_splits_total 45
# HELP database_observability_sql_server_process_available_commit_memory_bytes Memory available to be committed by the SQL Server process, in bytes.
# TYPE database_observability_sql_server_process_available_commit_memory_bytes gauge
database_observability_sql_server_process_available_commit_memory_bytes 1024
# HELP database_observability_sql_server_process_committed_memory_resident_ratio Ratio of SQL Server committed memory that is resident in the working set.
# TYPE database_observability_sql_server_process_committed_memory_resident_ratio gauge
database_observability_sql_server_process_committed_memory_resident_ratio 0.75
# HELP database_observability_sql_server_process_cpu_seconds_total Total CPU time consumed by SQL Server threads, in seconds.
# TYPE database_observability_sql_server_process_cpu_seconds_total counter
database_observability_sql_server_process_cpu_seconds_total{mode="kernel"} 30.25
database_observability_sql_server_process_cpu_seconds_total{mode="user"} 120.5
# HELP database_observability_sql_server_process_page_faults_total Total page faults incurred by the SQL Server process.
# TYPE database_observability_sql_server_process_page_faults_total counter
database_observability_sql_server_process_page_faults_total 42
# HELP database_observability_sql_server_process_physical_memory_low Whether the SQL Server process is responding to a low physical-memory notification.
# TYPE database_observability_sql_server_process_physical_memory_low gauge
database_observability_sql_server_process_physical_memory_low 0
# HELP database_observability_sql_server_process_resident_memory_bytes SQL Server process working set, in bytes.
# TYPE database_observability_sql_server_process_resident_memory_bytes gauge
database_observability_sql_server_process_resident_memory_bytes 3072
# HELP database_observability_sql_server_process_virtual_memory_bytes Virtual address space committed by the SQL Server process, in bytes.
# TYPE database_observability_sql_server_process_virtual_memory_bytes gauge
database_observability_sql_server_process_virtual_memory_bytes 4096
# HELP database_observability_sql_server_process_virtual_memory_low Whether the SQL Server process has detected a low virtual-memory condition.
# TYPE database_observability_sql_server_process_virtual_memory_low gauge
database_observability_sql_server_process_virtual_memory_low 1
# HELP database_observability_sql_server_processes_blocked Current number of blocked processes.
# TYPE database_observability_sql_server_processes_blocked gauge
database_observability_sql_server_processes_blocked 4
# HELP database_observability_sql_server_sql_compilations_total Total SQL compilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_compilations_total counter
database_observability_sql_server_sql_compilations_total 400
# HELP database_observability_sql_server_sql_recompilations_total Total SQL statement recompilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_recompilations_total counter
database_observability_sql_server_sql_recompilations_total 15
# HELP database_observability_sql_server_tempdb_space_unallocated_bytes Unallocated space in tempdb data files, in bytes.
# TYPE database_observability_sql_server_tempdb_space_unallocated_bytes gauge
database_observability_sql_server_tempdb_space_unallocated_bytes 524288
# HELP database_observability_sql_server_tempdb_space_used_bytes Space used in tempdb data files by allocation category, in bytes.
# TYPE database_observability_sql_server_tempdb_space_used_bytes gauge
database_observability_sql_server_tempdb_space_used_bytes{category="internal_objects"} 32768
database_observability_sql_server_tempdb_space_used_bytes{category="mixed_extents"} 8192
database_observability_sql_server_tempdb_space_used_bytes{category="user_objects"} 65536
database_observability_sql_server_tempdb_space_used_bytes{category="version_store"} 16384
# HELP database_observability_sql_server_transactions_total Total transactions started across all databases.
# TYPE database_observability_sql_server_transactions_total counter
database_observability_sql_server_transactions_total 1200
# HELP database_observability_sql_server_uptime_seconds Time since the SQL Server process started, in seconds.
# TYPE database_observability_sql_server_uptime_seconds gauge
database_observability_sql_server_uptime_seconds 3600
# HELP database_observability_sql_server_user_session_limit Maximum simultaneous SQL Server user sessions allowed by the active configuration and service limit.
# TYPE database_observability_sql_server_user_session_limit gauge
database_observability_sql_server_user_session_limit 32767
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 1
database_observability_sql_server_user_sessions{status="running"} 2
database_observability_sql_server_user_sessions{status="sleeping"} 3
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsAzureSQLManagedInstance(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: azureSQLManagedInstanceEngineEdition,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectServerMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(serverMetricsColumns).
			AddRow(120, 30, 3600, 64, 1024, 2048, 8192, 4096, 16384, 8192, 1, 0, 3072, 4096, 1024, 0.75, 42, 0, 0))
	mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(userSessionColumns).AddRow(5, 10, 0, 0, nil))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(databaseStorageColumns).AddRow(16000000000, 16800000000, 790000000, 830000000, nil))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(performanceCounterColumns).AddRow(
			1000, 300, 50, 2, 2400, 950, 1000, 10, 4, 1, 8, 20, 0, 1, 0, 4, 0.25, 15, 90, 14,
		))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(fileIOStatsColumns).AddRow(2048, 1024, 20, 10, 0.1, 0.2))
	mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(tempdbSpaceUsageColumns).AddRow(8192, 8192, 0, 0, 65536))
	mock.ExpectQuery(selectManagedInstanceMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{
			"process_cpu_limit",
			"process_memory_limit_bytes",
			"sos_memory_target_bytes",
		}).AddRow(8, 67108864, 50331648))
	resourceStart := time.Date(2026, time.September, 15, 14, 44, 16, 396666000, time.UTC)
	resourceEnd := resourceStart.Add(15020 * time.Millisecond)
	mock.ExpectQuery(selectManagedInstanceResourceMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(managedInstanceResourceMetricColumns).
			AddRow(resourceStart, resourceEnd, 4, 0.3527, 34359738368, 19326304256, 1502, 3004, 4506))

	expected := `
# HELP database_observability_sql_server_batch_requests_total Total Transact-SQL command batches received by SQL Server.
# TYPE database_observability_sql_server_batch_requests_total counter
database_observability_sql_server_batch_requests_total 1000
# HELP database_observability_sql_server_buffer_cache_hit_ratio Ratio of pages found in the buffer pool without reading from disk.
# TYPE database_observability_sql_server_buffer_cache_hit_ratio gauge
database_observability_sql_server_buffer_cache_hit_ratio 0.95
# HELP database_observability_sql_server_checkpoint_pages_total Total pages flushed to disk by checkpoint operations.
# TYPE database_observability_sql_server_checkpoint_pages_total counter
database_observability_sql_server_checkpoint_pages_total 10
# HELP database_observability_sql_server_connection_resets_total Total logins started from the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_connection_resets_total counter
database_observability_sql_server_connection_resets_total 90
# HELP database_observability_sql_server_database_storage_allocated_bytes Space allocated to the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_allocated_bytes gauge
database_observability_sql_server_database_storage_allocated_bytes{file_type="log"} 8.3e+08
database_observability_sql_server_database_storage_allocated_bytes{file_type="rows"} 1.68e+10
# HELP database_observability_sql_server_database_storage_used_bytes Space used within the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_used_bytes gauge
database_observability_sql_server_database_storage_used_bytes{file_type="log"} 7.9e+08
database_observability_sql_server_database_storage_used_bytes{file_type="rows"} 1.6e+10
# HELP database_observability_sql_server_deadlocks_total Total lock requests that resulted in a deadlock.
# TYPE database_observability_sql_server_deadlocks_total counter
database_observability_sql_server_deadlocks_total 0
# HELP database_observability_sql_server_free_list_stalls_total Total requests that had to wait for a free page in the buffer pool.
# TYPE database_observability_sql_server_free_list_stalls_total counter
database_observability_sql_server_free_list_stalls_total 1
# HELP database_observability_sql_server_full_scans_total Total unrestricted base-table or full-index scans performed by SQL Server.
# TYPE database_observability_sql_server_full_scans_total counter
database_observability_sql_server_full_scans_total 20
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="azure_sql_managed_instance",engine_edition="8",product_build="",product_level="",product_version=""} 1
# HELP database_observability_sql_server_instance_cpu_utilization_ratio Average CPU utilization relative to the Azure SQL service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_instance_cpu_utilization_ratio gauge
database_observability_sql_server_instance_cpu_utilization_ratio 0.3527
# HELP database_observability_sql_server_instance_io_operations_per_second Average physical I/O operations per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_operations_per_second gauge
database_observability_sql_server_instance_io_operations_per_second 100
# HELP database_observability_sql_server_instance_io_read_bytes_per_second Average physical bytes read per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_read_bytes_per_second gauge
database_observability_sql_server_instance_io_read_bytes_per_second 200
# HELP database_observability_sql_server_instance_io_written_bytes_per_second Average physical bytes written per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_written_bytes_per_second gauge
database_observability_sql_server_instance_io_written_bytes_per_second 300
# HELP database_observability_sql_server_instance_resource_stats_timestamp_seconds Unix timestamp of the latest Azure SQL Managed Instance resource statistics interval.
# TYPE database_observability_sql_server_instance_resource_stats_timestamp_seconds gauge
database_observability_sql_server_instance_resource_stats_timestamp_seconds 1.789483471416666e+09
# HELP database_observability_sql_server_instance_storage_limit_bytes Reserved storage capacity of the Azure SQL Managed Instance or total operating system volume capacity, in bytes.
# TYPE database_observability_sql_server_instance_storage_limit_bytes gauge
database_observability_sql_server_instance_storage_limit_bytes 3.4359738368e+10
# HELP database_observability_sql_server_instance_storage_used_bytes Storage used by the Azure SQL Managed Instance or across operating system volumes hosting SQL Server files, in bytes.
# TYPE database_observability_sql_server_instance_storage_used_bytes gauge
database_observability_sql_server_instance_storage_used_bytes 1.9326304256e+10
# HELP database_observability_sql_server_instance_vcore_limit vCore limit of the Azure SQL Managed Instance.
# TYPE database_observability_sql_server_instance_vcore_limit gauge
database_observability_sql_server_instance_vcore_limit 4
# HELP database_observability_sql_server_io_read_bytes_total Total bytes read from SQL Server database and log files.
# TYPE database_observability_sql_server_io_read_bytes_total counter
database_observability_sql_server_io_read_bytes_total 2048
# HELP database_observability_sql_server_io_read_stall_seconds_total Total time spent waiting for read operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_read_stall_seconds_total counter
database_observability_sql_server_io_read_stall_seconds_total 0.1
# HELP database_observability_sql_server_io_reads_total Total physical read operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_reads_total counter
database_observability_sql_server_io_reads_total 20
# HELP database_observability_sql_server_io_write_stall_seconds_total Total time spent waiting for write operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_write_stall_seconds_total counter
database_observability_sql_server_io_write_stall_seconds_total 0.2
# HELP database_observability_sql_server_io_writes_total Total physical write operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_writes_total counter
database_observability_sql_server_io_writes_total 10
# HELP database_observability_sql_server_io_written_bytes_total Total bytes written to SQL Server database and log files.
# TYPE database_observability_sql_server_io_written_bytes_total counter
database_observability_sql_server_io_written_bytes_total 1024
# HELP database_observability_sql_server_lazy_writes_total Total pages written to disk by the lazy writer to free buffer pool pages.
# TYPE database_observability_sql_server_lazy_writes_total counter
database_observability_sql_server_lazy_writes_total 4
# HELP database_observability_sql_server_lock_wait_seconds_total Total time spent waiting for locks, in seconds.
# TYPE database_observability_sql_server_lock_wait_seconds_total counter
database_observability_sql_server_lock_wait_seconds_total 0.25
# HELP database_observability_sql_server_lock_waits_total Total lock requests that required the caller to wait.
# TYPE database_observability_sql_server_lock_waits_total counter
database_observability_sql_server_lock_waits_total 4
# HELP database_observability_sql_server_logins_total Total logins started outside the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_logins_total counter
database_observability_sql_server_logins_total 15
# HELP database_observability_sql_server_logouts_total Total logout operations since SQL Server last started.
# TYPE database_observability_sql_server_logouts_total counter
database_observability_sql_server_logouts_total 14
# HELP database_observability_sql_server_memory_grants_pending Current number of processes waiting for a workspace memory grant.
# TYPE database_observability_sql_server_memory_grants_pending gauge
database_observability_sql_server_memory_grants_pending 0
# HELP database_observability_sql_server_memory_manager_committed_bytes Memory committed by the SQL Server memory manager, in bytes.
# TYPE database_observability_sql_server_memory_manager_committed_bytes gauge
database_observability_sql_server_memory_manager_committed_bytes 1024
# HELP database_observability_sql_server_memory_manager_target_bytes Memory the SQL Server memory manager can consume, in bytes.
# TYPE database_observability_sql_server_memory_manager_target_bytes gauge
database_observability_sql_server_memory_manager_target_bytes 2048
# HELP database_observability_sql_server_page_life_expectancy_seconds Duration in seconds that a data page stays in the buffer pool without references.
# TYPE database_observability_sql_server_page_life_expectancy_seconds gauge
database_observability_sql_server_page_life_expectancy_seconds 2400
# HELP database_observability_sql_server_page_splits_total Total index page splits performed by SQL Server.
# TYPE database_observability_sql_server_page_splits_total counter
database_observability_sql_server_page_splits_total 8
# HELP database_observability_sql_server_process_available_commit_memory_bytes Memory available to be committed by the SQL Server process, in bytes.
# TYPE database_observability_sql_server_process_available_commit_memory_bytes gauge
database_observability_sql_server_process_available_commit_memory_bytes 1024
# HELP database_observability_sql_server_process_committed_memory_resident_ratio Ratio of SQL Server committed memory that is resident in the working set.
# TYPE database_observability_sql_server_process_committed_memory_resident_ratio gauge
database_observability_sql_server_process_committed_memory_resident_ratio 0.75
# HELP database_observability_sql_server_process_cpu_limit Logical CPU capacity available to the Azure SQL Managed Instance process.
# TYPE database_observability_sql_server_process_cpu_limit gauge
database_observability_sql_server_process_cpu_limit 8
# HELP database_observability_sql_server_process_cpu_seconds_total Total CPU time consumed by SQL Server threads, in seconds.
# TYPE database_observability_sql_server_process_cpu_seconds_total counter
database_observability_sql_server_process_cpu_seconds_total{mode="kernel"} 30
database_observability_sql_server_process_cpu_seconds_total{mode="user"} 120
# HELP database_observability_sql_server_process_memory_limit_bytes Maximum committed memory available to the Azure SQL Managed Instance process, in bytes.
# TYPE database_observability_sql_server_process_memory_limit_bytes gauge
database_observability_sql_server_process_memory_limit_bytes 6.7108864e+07
# HELP database_observability_sql_server_process_page_faults_total Total page faults incurred by the SQL Server process.
# TYPE database_observability_sql_server_process_page_faults_total counter
database_observability_sql_server_process_page_faults_total 42
# HELP database_observability_sql_server_process_physical_memory_low Whether the SQL Server process is responding to a low physical-memory notification.
# TYPE database_observability_sql_server_process_physical_memory_low gauge
database_observability_sql_server_process_physical_memory_low 0
# HELP database_observability_sql_server_process_resident_memory_bytes SQL Server process working set, in bytes.
# TYPE database_observability_sql_server_process_resident_memory_bytes gauge
database_observability_sql_server_process_resident_memory_bytes 3072
# HELP database_observability_sql_server_process_virtual_memory_bytes Virtual address space committed by the SQL Server process, in bytes.
# TYPE database_observability_sql_server_process_virtual_memory_bytes gauge
database_observability_sql_server_process_virtual_memory_bytes 4096
# HELP database_observability_sql_server_process_virtual_memory_low Whether the SQL Server process has detected a low virtual-memory condition.
# TYPE database_observability_sql_server_process_virtual_memory_low gauge
database_observability_sql_server_process_virtual_memory_low 0
# HELP database_observability_sql_server_processes_blocked Current number of blocked processes.
# TYPE database_observability_sql_server_processes_blocked gauge
database_observability_sql_server_processes_blocked 1
# HELP database_observability_sql_server_sos_memory_target_bytes Memory available to the Azure SQL Managed Instance SQL operating system, in bytes.
# TYPE database_observability_sql_server_sos_memory_target_bytes gauge
database_observability_sql_server_sos_memory_target_bytes 5.0331648e+07
# HELP database_observability_sql_server_sql_compilations_total Total SQL compilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_compilations_total counter
database_observability_sql_server_sql_compilations_total 50
# HELP database_observability_sql_server_sql_recompilations_total Total SQL statement recompilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_recompilations_total counter
database_observability_sql_server_sql_recompilations_total 2
# HELP database_observability_sql_server_tempdb_space_unallocated_bytes Unallocated space in tempdb data files, in bytes.
# TYPE database_observability_sql_server_tempdb_space_unallocated_bytes gauge
database_observability_sql_server_tempdb_space_unallocated_bytes 65536
# HELP database_observability_sql_server_tempdb_space_used_bytes Space used in tempdb data files by allocation category, in bytes.
# TYPE database_observability_sql_server_tempdb_space_used_bytes gauge
database_observability_sql_server_tempdb_space_used_bytes{category="internal_objects"} 8192
database_observability_sql_server_tempdb_space_used_bytes{category="mixed_extents"} 0
database_observability_sql_server_tempdb_space_used_bytes{category="user_objects"} 8192
database_observability_sql_server_tempdb_space_used_bytes{category="version_store"} 0
# HELP database_observability_sql_server_transactions_total Total transactions started across all databases.
# TYPE database_observability_sql_server_transactions_total counter
database_observability_sql_server_transactions_total 300
# HELP database_observability_sql_server_uptime_seconds Time since the SQL Server process started, in seconds.
# TYPE database_observability_sql_server_uptime_seconds gauge
database_observability_sql_server_uptime_seconds 3600
# HELP database_observability_sql_server_user_session_limit Maximum simultaneous SQL Server user sessions allowed by the active configuration and service limit.
# TYPE database_observability_sql_server_user_session_limit gauge
database_observability_sql_server_user_session_limit 30000
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 0
database_observability_sql_server_user_sessions{status="running"} 5
database_observability_sql_server_user_sessions{status="sleeping"} 10
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsAzureSQLDatabase(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: azureSQLDatabaseEngineEdition,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	resourceEnd := time.Date(2026, time.September, 15, 14, 4, 44, 253000000, time.UTC)
	mock.ExpectQuery(selectAzureSQLDatabaseResourceMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseResourceMetricColumns).
			AddRow(resourceEnd, 0.9346, 0.67, 0.1128, 0.0013, 0.065, 0.0006, 0.5829, 0.7674))
	mock.ExpectQuery(selectAzureSQLDatabaseUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseUserSessionColumns).AddRow(14, 6, 0, 0))
	mock.ExpectQuery(selectAzureSQLDatabaseLimits).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseLimitColumns).AddRow(2, 0, 30000, 200, 1, 0.25))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(databaseStorageColumns).AddRow(18028126208, 18071117824, 0, 0, 34359738368))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(performanceCounterColumns).AddRow(
			250, 75, 10, 1, 900, 0, 0, 5, 2, 0, 3, 6, 0, 0, 0, 2, 0.1, 4, 20, 3,
		))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(fileIOStatsColumns).AddRow(4096, 2048, 8, 4, 0.05, 0.08))

	expected := `
# HELP database_observability_sql_server_batch_requests_total Total Transact-SQL command batches received by SQL Server.
# TYPE database_observability_sql_server_batch_requests_total counter
database_observability_sql_server_batch_requests_total 250
# HELP database_observability_sql_server_checkpoint_pages_total Total pages flushed to disk by checkpoint operations.
# TYPE database_observability_sql_server_checkpoint_pages_total counter
database_observability_sql_server_checkpoint_pages_total 5
# HELP database_observability_sql_server_connection_resets_total Total logins started from the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_connection_resets_total counter
database_observability_sql_server_connection_resets_total 20
# HELP database_observability_sql_server_database_cpu_utilization_ratio Average database CPU utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_cpu_utilization_ratio gauge
database_observability_sql_server_database_cpu_utilization_ratio 0.9346
# HELP database_observability_sql_server_database_data_io_utilization_ratio Average database data I/O utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_data_io_utilization_ratio gauge
database_observability_sql_server_database_data_io_utilization_ratio 0.1128
# HELP database_observability_sql_server_database_log_write_utilization_ratio Average database log-write utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_log_write_utilization_ratio gauge
database_observability_sql_server_database_log_write_utilization_ratio 0.0013
# HELP database_observability_sql_server_database_memory_grant_limit_ratio Maximum memory grant for a single Azure SQL Database request as a ratio of user workload group memory.
# TYPE database_observability_sql_server_database_memory_grant_limit_ratio gauge
database_observability_sql_server_database_memory_grant_limit_ratio 0.25
# HELP database_observability_sql_server_database_memory_utilization_ratio Average database memory utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_memory_utilization_ratio gauge
database_observability_sql_server_database_memory_utilization_ratio 0.67
# HELP database_observability_sql_server_database_resource_stats_timestamp_seconds Unix timestamp of the latest Azure SQL Database resource statistics interval.
# TYPE database_observability_sql_server_database_resource_stats_timestamp_seconds gauge
database_observability_sql_server_database_resource_stats_timestamp_seconds 1.789481084253e+09
# HELP database_observability_sql_server_database_resource_governor_max_degree_of_parallelism Maximum degree of parallelism enforced by the Azure SQL Database user workload group.
# TYPE database_observability_sql_server_database_resource_governor_max_degree_of_parallelism gauge
database_observability_sql_server_database_resource_governor_max_degree_of_parallelism 1
# HELP database_observability_sql_server_database_session_utilization_ratio Maximum database session utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_session_utilization_ratio gauge
database_observability_sql_server_database_session_utilization_ratio 0.0006
# HELP database_observability_sql_server_database_storage_allocated_bytes Space allocated to the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_allocated_bytes gauge
database_observability_sql_server_database_storage_allocated_bytes{file_type="log"} 0
database_observability_sql_server_database_storage_allocated_bytes{file_type="rows"} 1.8071117824e+10
# HELP database_observability_sql_server_database_storage_limit_bytes Maximum configured storage capacity of the connected database, in bytes.
# TYPE database_observability_sql_server_database_storage_limit_bytes gauge
database_observability_sql_server_database_storage_limit_bytes 3.4359738368e+10
# HELP database_observability_sql_server_database_storage_used_bytes Space used within the connected database files, in bytes.
# TYPE database_observability_sql_server_database_storage_used_bytes gauge
database_observability_sql_server_database_storage_used_bytes{file_type="log"} 0
database_observability_sql_server_database_storage_used_bytes{file_type="rows"} 1.8028126208e+10
# HELP database_observability_sql_server_database_vcore_limit vCore limit of the Azure SQL Database.
# TYPE database_observability_sql_server_database_vcore_limit gauge
database_observability_sql_server_database_vcore_limit 2
# HELP database_observability_sql_server_database_worker_limit Worker thread limit of the Azure SQL Database user workload group.
# TYPE database_observability_sql_server_database_worker_limit gauge
database_observability_sql_server_database_worker_limit 200
# HELP database_observability_sql_server_database_worker_utilization_ratio Maximum database worker utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_worker_utilization_ratio gauge
database_observability_sql_server_database_worker_utilization_ratio 0.065
# HELP database_observability_sql_server_deadlocks_total Total lock requests that resulted in a deadlock.
# TYPE database_observability_sql_server_deadlocks_total counter
database_observability_sql_server_deadlocks_total 0
# HELP database_observability_sql_server_free_list_stalls_total Total requests that had to wait for a free page in the buffer pool.
# TYPE database_observability_sql_server_free_list_stalls_total counter
database_observability_sql_server_free_list_stalls_total 0
# HELP database_observability_sql_server_full_scans_total Total unrestricted base-table or full-index scans performed by SQL Server.
# TYPE database_observability_sql_server_full_scans_total counter
database_observability_sql_server_full_scans_total 6
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="azure_sql_database",engine_edition="5",product_build="",product_level="",product_version=""} 1
# HELP database_observability_sql_server_instance_cpu_utilization_ratio Average CPU utilization relative to the Azure SQL service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_instance_cpu_utilization_ratio gauge
database_observability_sql_server_instance_cpu_utilization_ratio 0.5829
# HELP database_observability_sql_server_instance_memory_utilization_ratio Average memory utilization of the database engine instance hosting the Azure SQL Database during the latest reporting interval.
# TYPE database_observability_sql_server_instance_memory_utilization_ratio gauge
database_observability_sql_server_instance_memory_utilization_ratio 0.7674
# HELP database_observability_sql_server_io_read_bytes_total Total bytes read from SQL Server database and log files.
# TYPE database_observability_sql_server_io_read_bytes_total counter
database_observability_sql_server_io_read_bytes_total 4096
# HELP database_observability_sql_server_io_read_stall_seconds_total Total time spent waiting for read operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_read_stall_seconds_total counter
database_observability_sql_server_io_read_stall_seconds_total 0.05
# HELP database_observability_sql_server_io_reads_total Total physical read operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_reads_total counter
database_observability_sql_server_io_reads_total 8
# HELP database_observability_sql_server_io_write_stall_seconds_total Total time spent waiting for write operations on SQL Server files, in seconds.
# TYPE database_observability_sql_server_io_write_stall_seconds_total counter
database_observability_sql_server_io_write_stall_seconds_total 0.08
# HELP database_observability_sql_server_io_writes_total Total physical write operations issued on SQL Server database and log files.
# TYPE database_observability_sql_server_io_writes_total counter
database_observability_sql_server_io_writes_total 4
# HELP database_observability_sql_server_io_written_bytes_total Total bytes written to SQL Server database and log files.
# TYPE database_observability_sql_server_io_written_bytes_total counter
database_observability_sql_server_io_written_bytes_total 2048
# HELP database_observability_sql_server_lazy_writes_total Total pages written to disk by the lazy writer to free buffer pool pages.
# TYPE database_observability_sql_server_lazy_writes_total counter
database_observability_sql_server_lazy_writes_total 2
# HELP database_observability_sql_server_lock_wait_seconds_total Total time spent waiting for locks, in seconds.
# TYPE database_observability_sql_server_lock_wait_seconds_total counter
database_observability_sql_server_lock_wait_seconds_total 0.1
# HELP database_observability_sql_server_lock_waits_total Total lock requests that required the caller to wait.
# TYPE database_observability_sql_server_lock_waits_total counter
database_observability_sql_server_lock_waits_total 2
# HELP database_observability_sql_server_logins_total Total logins started outside the connection pool since SQL Server last started.
# TYPE database_observability_sql_server_logins_total counter
database_observability_sql_server_logins_total 4
# HELP database_observability_sql_server_logouts_total Total logout operations since SQL Server last started.
# TYPE database_observability_sql_server_logouts_total counter
database_observability_sql_server_logouts_total 3
# HELP database_observability_sql_server_memory_grants_pending Current number of processes waiting for a workspace memory grant.
# TYPE database_observability_sql_server_memory_grants_pending gauge
database_observability_sql_server_memory_grants_pending 0
# HELP database_observability_sql_server_page_life_expectancy_seconds Duration in seconds that a data page stays in the buffer pool without references.
# TYPE database_observability_sql_server_page_life_expectancy_seconds gauge
database_observability_sql_server_page_life_expectancy_seconds 900
# HELP database_observability_sql_server_page_splits_total Total index page splits performed by SQL Server.
# TYPE database_observability_sql_server_page_splits_total counter
database_observability_sql_server_page_splits_total 3
# HELP database_observability_sql_server_processes_blocked Current number of blocked processes.
# TYPE database_observability_sql_server_processes_blocked gauge
database_observability_sql_server_processes_blocked 0
# HELP database_observability_sql_server_sql_compilations_total Total SQL compilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_compilations_total counter
database_observability_sql_server_sql_compilations_total 10
# HELP database_observability_sql_server_sql_recompilations_total Total SQL statement recompilations performed by SQL Server.
# TYPE database_observability_sql_server_sql_recompilations_total counter
database_observability_sql_server_sql_recompilations_total 1
# HELP database_observability_sql_server_transactions_total Total transactions started across all databases.
# TYPE database_observability_sql_server_transactions_total counter
database_observability_sql_server_transactions_total 75
# HELP database_observability_sql_server_user_session_limit Maximum simultaneous SQL Server user sessions allowed by the active configuration and service limit.
# TYPE database_observability_sql_server_user_session_limit gauge
database_observability_sql_server_user_session_limit 30000
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 0
database_observability_sql_server_user_sessions{status="running"} 14
database_observability_sql_server_user_sessions{status="sleeping"} 6
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsAzureSQLDatabaseDTULimitWithoutResourceHistory(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: azureSQLDatabaseEngineEdition,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectAzureSQLDatabaseResourceMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseResourceMetricColumns))
	mock.ExpectQuery(selectAzureSQLDatabaseUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseUserSessionColumns).AddRow(0, 0, 0, 0))
	mock.ExpectQuery(selectAzureSQLDatabaseLimits).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseLimitColumns).AddRow(nil, 100, 30000, 200, 1, 0.25))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().WillReturnError(fmt.Errorf("storage unavailable"))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().WillReturnError(fmt.Errorf("counters unavailable"))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().WillReturnError(fmt.Errorf("io stats unavailable"))

	expected := `
# HELP database_observability_sql_server_database_dtu_limit DTU limit of the Azure SQL Database.
# TYPE database_observability_sql_server_database_dtu_limit gauge
database_observability_sql_server_database_dtu_limit 100
`

	require.NoError(t, testutil.CollectAndCompare(
		registry,
		strings.NewReader(expected),
		"database_observability_sql_server_database_dtu_limit",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsAzureSQLDatabaseResourceFailureDoesNotStopSessions(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: azureSQLDatabaseEngineEdition,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectAzureSQLDatabaseResourceMetrics).WithoutArgs().
		WillReturnError(fmt.Errorf("resource metrics unavailable"))
	mock.ExpectQuery(selectAzureSQLDatabaseUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(azureSQLDatabaseUserSessionColumns).AddRow(2, 3, 0, 0))
	mock.ExpectQuery(selectAzureSQLDatabaseLimits).WithoutArgs().
		WillReturnError(fmt.Errorf("resource limits unavailable"))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().
		WillReturnError(fmt.Errorf("storage unavailable"))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().
		WillReturnError(fmt.Errorf("counters unavailable"))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().
		WillReturnError(fmt.Errorf("io stats unavailable"))

	expected := `
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 0
database_observability_sql_server_user_sessions{status="running"} 2
database_observability_sql_server_user_sessions{status="sleeping"} 3
`

	require.NoError(t, testutil.CollectAndCompare(
		registry,
		strings.NewReader(expected),
		"database_observability_sql_server_user_sessions",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManagedInstanceResourceMetricsSurviveProtectedQueryFailures(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: azureSQLManagedInstanceEngineEdition,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectServerMetrics).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
	mock.ExpectQuery(selectManagedInstanceMetrics).WithoutArgs().
		WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))

	resourceStart := time.Date(2026, time.September, 15, 14, 44, 16, 396666000, time.UTC)
	resourceEnd := resourceStart.Add(15020 * time.Millisecond)
	mock.ExpectQuery(selectManagedInstanceResourceMetrics).WithoutArgs().RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows(managedInstanceResourceMetricColumns).
			AddRow(resourceStart, resourceEnd, 4, 0.3527, 34359738368, 19326304256, 1502, 3004, 4506))

	expected := `
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="azure_sql_managed_instance",engine_edition="8",product_build="",product_level="",product_version=""} 1
# HELP database_observability_sql_server_instance_cpu_utilization_ratio Average CPU utilization relative to the Azure SQL service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_instance_cpu_utilization_ratio gauge
database_observability_sql_server_instance_cpu_utilization_ratio 0.3527
# HELP database_observability_sql_server_instance_io_operations_per_second Average physical I/O operations per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_operations_per_second gauge
database_observability_sql_server_instance_io_operations_per_second 100
# HELP database_observability_sql_server_instance_io_read_bytes_per_second Average physical bytes read per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_read_bytes_per_second gauge
database_observability_sql_server_instance_io_read_bytes_per_second 200
# HELP database_observability_sql_server_instance_io_written_bytes_per_second Average physical bytes written per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_written_bytes_per_second gauge
database_observability_sql_server_instance_io_written_bytes_per_second 300
# HELP database_observability_sql_server_instance_resource_stats_timestamp_seconds Unix timestamp of the latest Azure SQL Managed Instance resource statistics interval.
# TYPE database_observability_sql_server_instance_resource_stats_timestamp_seconds gauge
database_observability_sql_server_instance_resource_stats_timestamp_seconds 1.789483471416666e+09
# HELP database_observability_sql_server_instance_storage_limit_bytes Reserved storage capacity of the Azure SQL Managed Instance or total operating system volume capacity, in bytes.
# TYPE database_observability_sql_server_instance_storage_limit_bytes gauge
database_observability_sql_server_instance_storage_limit_bytes 3.4359738368e+10
# HELP database_observability_sql_server_instance_storage_used_bytes Storage used by the Azure SQL Managed Instance or across operating system volumes hosting SQL Server files, in bytes.
# TYPE database_observability_sql_server_instance_storage_used_bytes gauge
database_observability_sql_server_instance_storage_used_bytes 1.9326304256e+10
# HELP database_observability_sql_server_instance_vcore_limit vCore limit of the Azure SQL Managed Instance.
# TYPE database_observability_sql_server_instance_vcore_limit gauge
database_observability_sql_server_instance_vcore_limit 4
`

	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	assert.False(t, c.Stopped())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsNullableAzureResourceStats(t *testing.T) {
	t.Run("Azure SQL Database omits NULL resource stats columns without failing scrape", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		registry := prometheus.NewRegistry()
		c, err := NewServerMetrics(ServerMetricsArguments{
			DB:            db,
			Registry:      registry,
			Logger:        util.TestAlloyLogger(t).Slog(),
			EngineEdition: azureSQLDatabaseEngineEdition,
		})
		require.NoError(t, err)
		require.NoError(t, c.Start(t.Context()))
		defer c.Stop()

		resourceEnd := time.Date(2026, time.September, 15, 14, 4, 44, 253000000, time.UTC)
		mock.ExpectQuery(selectAzureSQLDatabaseResourceMetrics).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(azureSQLDatabaseResourceMetricColumns).
				AddRow(resourceEnd, 0.5, 0.25, nil, nil, nil, nil, nil, nil))
		mock.ExpectQuery(selectAzureSQLDatabaseUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(azureSQLDatabaseUserSessionColumns).AddRow(1, 0, 0, 0))
		mock.ExpectQuery(selectAzureSQLDatabaseLimits).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(azureSQLDatabaseLimitColumns))
		mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().
			WillReturnError(fmt.Errorf("storage unavailable"))
		mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().
			WillReturnError(fmt.Errorf("counters unavailable"))
		mock.ExpectQuery(selectFileIOStats).WithoutArgs().
			WillReturnError(fmt.Errorf("io stats unavailable"))

		expected := `
# HELP database_observability_sql_server_database_cpu_utilization_ratio Average database CPU utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_cpu_utilization_ratio gauge
database_observability_sql_server_database_cpu_utilization_ratio 0.5
# HELP database_observability_sql_server_database_memory_utilization_ratio Average database memory utilization relative to the Azure SQL Database service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_database_memory_utilization_ratio gauge
database_observability_sql_server_database_memory_utilization_ratio 0.25
# HELP database_observability_sql_server_database_resource_stats_timestamp_seconds Unix timestamp of the latest Azure SQL Database resource statistics interval.
# TYPE database_observability_sql_server_database_resource_stats_timestamp_seconds gauge
database_observability_sql_server_database_resource_stats_timestamp_seconds 1.789481084253e+09
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="azure_sql_database",engine_edition="5",product_build="",product_level="",product_version=""} 1
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 0
database_observability_sql_server_user_sessions{status="running"} 1
database_observability_sql_server_user_sessions{status="sleeping"} 0
`

		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Managed Instance omits NULL resource stats columns without failing scrape", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		registry := prometheus.NewRegistry()
		c, err := NewServerMetrics(ServerMetricsArguments{
			DB:            db,
			Registry:      registry,
			Logger:        util.TestAlloyLogger(t).Slog(),
			EngineEdition: azureSQLManagedInstanceEngineEdition,
		})
		require.NoError(t, err)
		require.NoError(t, c.Start(t.Context()))
		defer c.Stop()

		mock.ExpectQuery(selectServerMetrics).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectFileIOStats).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))
		mock.ExpectQuery(selectManagedInstanceMetrics).WithoutArgs().
			WillReturnError(fmt.Errorf("VIEW SERVER PERFORMANCE STATE denied"))

		resourceStart := time.Date(2026, time.September, 15, 14, 44, 16, 396666000, time.UTC)
		resourceEnd := resourceStart.Add(15020 * time.Millisecond)
		mock.ExpectQuery(selectManagedInstanceResourceMetrics).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(managedInstanceResourceMetricColumns).
				AddRow(resourceStart, resourceEnd, 4, 0.3527, nil, nil, 1502, nil, nil))

		expected := `
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="azure_sql_managed_instance",engine_edition="8",product_build="",product_level="",product_version=""} 1
# HELP database_observability_sql_server_instance_cpu_utilization_ratio Average CPU utilization relative to the Azure SQL service-tier limit during the latest reporting interval.
# TYPE database_observability_sql_server_instance_cpu_utilization_ratio gauge
database_observability_sql_server_instance_cpu_utilization_ratio 0.3527
# HELP database_observability_sql_server_instance_io_operations_per_second Average physical I/O operations per second during the latest Azure SQL Managed Instance reporting interval.
# TYPE database_observability_sql_server_instance_io_operations_per_second gauge
database_observability_sql_server_instance_io_operations_per_second 100
# HELP database_observability_sql_server_instance_resource_stats_timestamp_seconds Unix timestamp of the latest Azure SQL Managed Instance resource statistics interval.
# TYPE database_observability_sql_server_instance_resource_stats_timestamp_seconds gauge
database_observability_sql_server_instance_resource_stats_timestamp_seconds 1.789483471416666e+09
# HELP database_observability_sql_server_instance_vcore_limit vCore limit of the Azure SQL Managed Instance.
# TYPE database_observability_sql_server_instance_vcore_limit gauge
database_observability_sql_server_instance_vcore_limit 4
`

		require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestServerMetricsQueryFailureDoesNotStopCollector(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: 3,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))
	defer c.Stop()

	mock.ExpectQuery(selectServerMetrics).WithoutArgs().WillReturnError(fmt.Errorf("server metrics unavailable"))
	mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).WillReturnError(fmt.Errorf("session metrics unavailable"))
	mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().WillReturnError(fmt.Errorf("storage unavailable"))
	mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().WillReturnError(fmt.Errorf("counters unavailable"))
	mock.ExpectQuery(selectFileIOStats).WithoutArgs().WillReturnError(fmt.Errorf("io stats unavailable"))
	mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().WillReturnError(fmt.Errorf("tempdb unavailable"))
	mock.ExpectQuery(selectVolumeStorage).WithoutArgs().WillReturnError(fmt.Errorf("volume storage unavailable"))

	expected := `
# HELP database_observability_sql_server_info Information about the SQL Server engine edition and product version.
# TYPE database_observability_sql_server_info gauge
database_observability_sql_server_info{compatibility_level="",edition_name="enterprise",engine_edition="3",product_build="",product_level="",product_version=""} 1
`
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(expected)))
	assert.False(t, c.Stopped())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestServerMetricsGiveEachQueryAnIndependentTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		registry := prometheus.NewRegistry()
		c, err := NewServerMetrics(ServerMetricsArguments{
			DB:            db,
			Registry:      registry,
			Logger:        util.TestAlloyLogger(t).Slog(),
			EngineEdition: 3,
			QueryTimeout:  time.Second,
		})
		require.NoError(t, err)
		require.NoError(t, c.Start(t.Context()))
		defer c.Stop()

		mock.ExpectQuery(selectServerMetrics).WithoutArgs().
			WillDelayFor(time.Hour).
			WillReturnRows(sqlmock.NewRows(serverMetricsColumns))
		mock.ExpectQuery(selectUserSessions).WithArgs(sql.Named("exclude_current_user", false), sql.Named("collector_app_name", AppName)).RowsWillBeClosed().
			WillReturnRows(sqlmock.NewRows(userSessionColumns).
				AddRow(2, 3, 0, 0, 100))
		mock.ExpectQuery(selectDatabaseStorage).WithoutArgs().WillReturnError(fmt.Errorf("storage unavailable"))
		mock.ExpectQuery(selectPerformanceCounters).WithoutArgs().WillReturnError(fmt.Errorf("counters unavailable"))
		mock.ExpectQuery(selectFileIOStats).WithoutArgs().WillReturnError(fmt.Errorf("io stats unavailable"))
		mock.ExpectQuery(selectTempdbSpaceUsage).WithoutArgs().WillReturnError(fmt.Errorf("tempdb unavailable"))
		mock.ExpectQuery(selectVolumeStorage).WithoutArgs().WillReturnError(fmt.Errorf("volume storage unavailable"))

		expected := `
# HELP database_observability_sql_server_user_session_limit Maximum simultaneous SQL Server user sessions allowed by the active configuration and service limit.
# TYPE database_observability_sql_server_user_session_limit gauge
database_observability_sql_server_user_session_limit 100
# HELP database_observability_sql_server_user_sessions Current authenticated SQL Server user sessions.
# TYPE database_observability_sql_server_user_sessions gauge
database_observability_sql_server_user_sessions{status="dormant"} 0
database_observability_sql_server_user_sessions{status="preconnect"} 0
database_observability_sql_server_user_sessions{status="running"} 2
database_observability_sql_server_user_sessions{status="sleeping"} 3
`

		start := time.Now()
		require.NoError(t, testutil.CollectAndCompare(
			registry,
			strings.NewReader(expected),
			"database_observability_sql_server_user_session_limit",
			"database_observability_sql_server_user_sessions",
		))
		// Only the delayed query waits, so one timeout elapses in total.
		require.Equal(t, time.Second, time.Since(start))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestEffectiveUserSessionLimit(t *testing.T) {
	testCases := []struct {
		name            string
		configuredLimit int
		engineEdition   int
		expected        int
	}{
		{
			name:            "self-configuring SQL Server default",
			configuredLimit: 0,
			engineEdition:   3,
			expected:        defaultUserSessionLimit,
		},
		{
			name:            "custom SQL Server limit",
			configuredLimit: 500,
			engineEdition:   3,
			expected:        500,
		},
		{
			name:            "Managed Instance service limit",
			configuredLimit: 0,
			engineEdition:   azureSQLManagedInstanceEngineEdition,
			expected:        azureSQLManagedInstanceSessionLimit,
		},
		{
			name:            "custom Managed Instance limit below service limit",
			configuredLimit: 500,
			engineEdition:   azureSQLManagedInstanceEngineEdition,
			expected:        500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, effectiveUserSessionLimit(tc.configuredLimit, tc.engineEdition))
		})
	}
}

func TestEditionNameFromEngineEdition(t *testing.T) {
	assert.Equal(t, "personal", editionNameFromEngineEdition(1))
	assert.Equal(t, "standard", editionNameFromEngineEdition(2))
	assert.Equal(t, "enterprise", editionNameFromEngineEdition(3))
	assert.Equal(t, "express", editionNameFromEngineEdition(4))
	assert.Equal(t, "azure_sql_database", editionNameFromEngineEdition(5))
	assert.Equal(t, "azure_synapse_analytics", editionNameFromEngineEdition(6))
	assert.Equal(t, "azure_sql_managed_instance", editionNameFromEngineEdition(8))
	assert.Equal(t, "azure_sql_edge", editionNameFromEngineEdition(9))
	assert.Equal(t, "azure_synapse_serverless", editionNameFromEngineEdition(11))
	assert.Equal(t, "unknown", editionNameFromEngineEdition(0))
}

func TestServerMetricsStopCancelsCollectContext(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	c, err := NewServerMetrics(ServerMetricsArguments{
		DB:            db,
		Registry:      registry,
		Logger:        util.TestAlloyLogger(t).Slog(),
		EngineEdition: 3,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))

	c.Stop()
	assert.True(t, c.Stopped())

	ch := make(chan prometheus.Metric, 8)
	c.Collect(ch)
	assert.Empty(t, ch)
	require.NoError(t, mock.ExpectationsWereMet())
}
