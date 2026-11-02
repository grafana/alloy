package collector

// Define SQL constants and templates here, use the Marker, and add new
// queries to TestQueryMarkers in queries_test.go. For dynamic SQL, test the
// builder output too. This is a review convention, not automatic enforcement.

// AppName identifies collector-owned connections in sys.dm_exec_sessions.program_name.
const AppName = "grafana:alloy:database_observability"

// QueryMarker identifies collector-owned statements in Query Store and live requests.
// Keep it stable across instances and versions so historical queries remain identifiable.
const QueryMarker = "/* " + AppName + " */"

// pingQuery is the connectivity check this component uses in place of the
// driver's untagged SELECT 1, so liveness probes carry the collector marker.
const PingQuery = "SELECT " + QueryMarker + " 1"

// selectServerInfo returns a stable identifier for the server instance plus
// its product version, product level, and engine edition, used to derive a
// server_id label and to expose version/edition metadata on metrics.
const SelectServerInfo = `SELECT ` + QueryMarker + `
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ServerName')) AS server_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('MachineName')) AS machine_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('InstanceName')) AS instance_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ProductVersion')) AS product_version,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ProductLevel')) AS product_level,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ProductBuild')) AS product_build,
    CONVERT(INT, SERVERPROPERTY('EngineEdition')) AS engine_edition,
    CONVERT(NVARCHAR(128), (SELECT compatibility_level FROM sys.databases WHERE database_id = DB_ID())) AS compatibility_level`

const SelectOriginalLogin = `SELECT ` + QueryMarker + ` ORIGINAL_LOGIN()`

// selectQueryMetrics ranks the top-N queries by duration within the recent lookback
// window, then sums their full retained Query Store history so the emitted
// counters stay monotonic across interval rollover.
//
// Note: the final SELECT sums the full retained history per ranked hash
// (no lookback filter on the aggregation), while the lookback window only
// constrains the top-N ranking/eligibility in the eligible/ranked CTEs.
// In internal state, on a Query Store bucket rollover (when interval ends),
// the cumulative totals keep growing → deltas stay non-negative → the
// Prometheus counters remain monotonic. The only time totals decrease
// is when Query Store is cleared, or when retention/cleanup drops an old
// interval: in that case the state re-baselines rather than emitting a spike.
const selectQueryMetrics = `
WITH filtered_queries AS (
	SELECT ` + QueryMarker + ` q.*
	FROM sys.query_store_query q
	JOIN sys.query_store_query_text qt ON qt.query_text_id = q.query_text_id
	WHERE (@exclude_current_user = 0 OR CHARINDEX(@collector_marker, qt.query_sql_text) = 0)
),
eligible AS (
	SELECT DISTINCT q.query_hash
	FROM filtered_queries q
	WHERE q.is_internal_query = 0
		AND q.query_hash IS NOT NULL
		AND q.last_execution_time >= DATEADD(SECOND, -CONVERT(int, @lookback_window), SYSUTCDATETIME())
),
ranked AS (
	SELECT TOP (@limit) q.query_hash
	FROM filtered_queries q
	JOIN eligible e ON e.query_hash = q.query_hash
	JOIN sys.query_store_plan p ON p.query_id = q.query_id
	JOIN sys.query_store_runtime_stats rs ON rs.plan_id = p.plan_id
	JOIN sys.query_store_runtime_stats_interval i
		ON i.runtime_stats_interval_id = rs.runtime_stats_interval_id
	WHERE q.is_internal_query = 0
		AND i.end_time >= DATEADD(SECOND, -CONVERT(int, @lookback_window), SYSUTCDATETIME())
	GROUP BY q.query_hash
	ORDER BY SUM(CONVERT(float, rs.avg_duration) * CONVERT(float, rs.count_executions)) DESC,
		q.query_hash ASC
)
SELECT
	q.query_hash,
	SUM(CONVERT(bigint, rs.count_executions)) AS executions,
	SUM(CASE
		WHEN rs.execution_type IN (3, 4)
		THEN CONVERT(bigint, rs.count_executions)
		ELSE 0 END) AS errors,
	SUM(CONVERT(float, rs.avg_duration) * CONVERT(float, rs.count_executions)) AS total_duration_us
FROM filtered_queries q
JOIN ranked r ON r.query_hash = q.query_hash
JOIN sys.query_store_plan p ON p.query_id = q.query_id
JOIN sys.query_store_runtime_stats rs ON rs.plan_id = p.plan_id
WHERE q.is_internal_query = 0
GROUP BY q.query_hash
ORDER BY q.query_hash ASC`

// selectQueryTextTemplate returns the query text for the tracked
// query_hashes on the currently connected database.
//
// TODO(cristian): consider fetching only the first query_sql_text
// and also using a cursor.
const selectQueryTextTemplate = `
	SELECT ` + QueryMarker + `
		q.query_hash,
		qt.query_sql_text
	FROM
		sys.query_store_query q
	JOIN
		sys.query_store_query_text qt ON qt.query_text_id = q.query_text_id
	WHERE
		q.is_internal_query = 0
		AND q.query_hash IN (%s)
		AND (@exclude_current_user = 0 OR CHARINDEX(@collector_marker, qt.query_sql_text) = 0)`

// selectExplainPlansTemplate resolves each tracked query_hash to the most
// recently active plan Query Store has already captured for it (no live
// SET SHOWPLAN_XML ON compilation - the plan is read from the same Query
// Store data query_metrics already joins). A query_hash can map to more than
// one query_id/plan_id (recompiles, parameter sniffing), so ROW_NUMBER picks
// exactly one plan per hash rather than enumerating every match. Recency is
// ranked by the plan's own last_execution_time rather than the latest
// runtime-stats interval end time, since that avoids joining through
// runtime_stats/runtime_stats_interval entirely - see the plan_id-keyed fetch
// below. plan_id DESC breaks ties (including NULL last_execution_time for a
// plan that has never executed, which sorts last under DESC) deterministically.
const selectExplainPlansTemplate = `
	WITH ranked_plans AS (
		SELECT ` + QueryMarker + `
			q.query_hash,
			p.plan_id,
			ROW_NUMBER() OVER (
				PARTITION BY q.query_hash
				ORDER BY p.last_execution_time DESC, p.plan_id DESC
			) AS rn
		FROM sys.query_store_query q
		JOIN sys.query_store_query_text qt ON qt.query_text_id = q.query_text_id
		JOIN sys.query_store_plan p ON p.query_id = q.query_id
		WHERE q.is_internal_query = 0
			AND q.query_hash IN (%s)
			AND (@exclude_current_user = 0 OR CHARINDEX(@collector_marker, qt.query_sql_text) = 0)
	)
	SELECT rp.query_hash, p.query_plan
	FROM ranked_plans rp
	JOIN sys.query_store_plan p ON p.plan_id = rp.plan_id
	WHERE rp.rn = 1`

const selectQuerySamplesTemplate = `
WITH tracked_requests AS (
	SELECT ` + QueryMarker + `
		SYSDATETIMEOFFSET() AS now,
		DB_NAME(r.database_id) AS database_name,
		r.session_id,
		r.request_id,
		TODATETIMEOFFSET(r.start_time, DATEPART(TZOFFSET, SYSDATETIMEOFFSET())) AS start_time,
		s.login_name,
		s.original_login_name,
		s.host_name,
		s.program_name,
		c.client_net_address,
		c.client_tcp_port,
		r.query_hash,
		r.cpu_time,
		r.total_elapsed_time,
		r.reads,
		r.writes,
		r.logical_reads,
		r.row_count,
		SUBSTRING(
			st.text,
			(r.statement_start_offset / 2) + 1,
			(
				CASE r.statement_end_offset
					WHEN -1 THEN DATALENGTH(st.text)
					ELSE r.statement_end_offset
				END - r.statement_start_offset
			) / 2 + 1
		) AS statement_text
	FROM sys.dm_exec_requests AS r
	INNER JOIN sys.dm_exec_sessions AS s
		ON s.session_id = r.session_id
	LEFT JOIN sys.dm_exec_connections AS c
		ON c.connection_id = r.connection_id
	OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) AS st
	WHERE
		r.session_id <> @@SPID
		AND s.is_user_process = 1
		AND r.database_id = DB_ID()
		AND r.query_hash IN (%s)
		AND (@exclude_current_user = 0 OR s.program_name IS NULL OR s.program_name <> @collector_app_name)
		%s
),
current_waits AS (
	SELECT
		t.session_id,
		t.request_id,
		w.exec_context_id,
		w.wait_type,
		w.wait_duration_ms,
		w.blocking_session_id,
		w.resource_description
	FROM sys.dm_os_tasks AS t
	INNER JOIN sys.dm_os_waiting_tasks AS w
		ON w.waiting_task_address = t.task_address
)
SELECT
	r.now,
	r.database_name,
	r.session_id,
	r.request_id,
	r.start_time,
	r.login_name,
	r.original_login_name,
	r.host_name,
	r.program_name,
	r.client_net_address,
	r.client_tcp_port,
	r.query_hash,
	r.cpu_time,
	r.total_elapsed_time,
	r.reads,
	r.writes,
	r.logical_reads,
	r.row_count,
	r.statement_text,
	w.exec_context_id,
	w.wait_type,
	w.wait_duration_ms,
	w.blocking_session_id,
	w.resource_description
FROM tracked_requests AS r
LEFT JOIN current_waits AS w
	ON w.session_id = r.session_id
	AND w.request_id = r.request_id
WHERE (@exclude_current_user = 0 OR r.statement_text IS NULL OR CHARINDEX(@collector_marker, r.statement_text) = 0)
ORDER BY r.session_id, r.request_id, w.exec_context_id`

// selectQueryStoreState reads the connected database's Query Store state
const selectQueryStoreState = `
	SELECT ` + QueryMarker + `
		DB_NAME(),
		actual_state_desc,
		query_capture_mode_desc,
		readonly_reason
	FROM sys.database_query_store_options`

const selectMyPermissionsQuery = `SELECT ` + QueryMarker + ` permission_name FROM sys.fn_my_permissions(NULL, 'DATABASE')`

const selectQueryStoreHasRowsQuery = `SELECT ` + QueryMarker + ` TOP 1 1 FROM sys.query_store_runtime_stats`

const (
	selectDatabasesTemplate = `
		SELECT ` + QueryMarker + ` name, QUOTENAME(name)
		FROM sys.databases
		WHERE state_desc = 'ONLINE'
			AND HAS_DBACCESS(name) = 1
			AND name NOT IN %s
		ORDER BY name`

	selectTablesTemplate = `
		SELECT ` + QueryMarker + `
			table_catalog,
			table_schema,
			table_name,
			table_type
		FROM information_schema.tables
		WHERE table_schema NOT IN %s
		ORDER BY table_schema, table_name`

	// selectColumnsTemplate returns columns information for every table in
	// the given IN-clause of table names, scoped to a single schema.
	// Note that column_type is emitted as the bare SQL Server type family name
	// (e.g. `varchar`, `int`, `decimal`) without precision/length.
	selectColumnsTemplate = `
		SELECT ` + QueryMarker + `
			OBJECT_NAME(c.object_id) AS table_name,
			c.name AS column_name,
			TYPE_NAME(c.user_type_id) AS column_type,
			c.is_nullable,
			c.is_identity,
			dc.definition AS default_value,
			CASE WHEN pk_ic.column_id IS NOT NULL THEN 1 ELSE 0 END AS is_primary_key
		FROM sys.columns c
		INNER JOIN sys.objects o ON c.object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		LEFT JOIN sys.default_constraints dc ON c.default_object_id = dc.object_id
		LEFT JOIN sys.indexes pk ON pk.object_id = c.object_id AND pk.is_primary_key = 1
		LEFT JOIN sys.index_columns pk_ic ON pk_ic.object_id = c.object_id AND pk_ic.index_id = pk.index_id AND pk_ic.column_id = c.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(c.object_id) IN %s
		ORDER BY OBJECT_NAME(c.object_id), c.column_id`

	// selectIndexesTemplate returns index information for every table in
	// the IN-clause. Note we filter out:
	// - Heaps (i.type = 0) and unnamed rowstore rows
	// - INCLUDE columns (ic.is_included_column = 1)
	selectIndexesTemplate = `
		SELECT ` + QueryMarker + `
			OBJECT_NAME(i.object_id) AS table_name,
			i.name AS index_name,
			ic.key_ordinal AS seq_in_index,
			c.name AS column_name,
			i.type_desc AS index_type,
			i.is_unique,
			c.is_nullable
		FROM sys.indexes i
		INNER JOIN sys.objects o ON i.object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id AND ic.is_included_column = 0
		INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(i.object_id) IN %s
			AND i.type > 0
			AND i.name IS NOT NULL
		ORDER BY OBJECT_NAME(i.object_id), i.name, ic.key_ordinal`

	// selectForeignKeysTemplate returns foreign key information for every table in
	// the IN-clause. Ordering by constraint_column_id preserves the correlation
	// between parent and referenced columns in composite foreign keys.
	selectForeignKeysTemplate = `
		SELECT ` + QueryMarker + `
			OBJECT_NAME(fk.parent_object_id) AS table_name,
			fk.name AS constraint_name,
			pcol.name AS column_name,
			OBJECT_NAME(fk.referenced_object_id) AS referenced_table_name,
			rcol.name AS referenced_column_name
		FROM sys.foreign_keys fk
		INNER JOIN sys.objects o ON fk.parent_object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		INNER JOIN sys.columns pcol ON fkc.parent_object_id = pcol.object_id AND fkc.parent_column_id = pcol.column_id
		INNER JOIN sys.columns rcol ON fkc.referenced_object_id = rcol.object_id AND fkc.referenced_column_id = rcol.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(fk.parent_object_id) IN %s
		ORDER BY OBJECT_NAME(fk.parent_object_id), fk.name, fkc.constraint_column_id`
)

const useDatabaseTemplate = "USE " + QueryMarker + " %s"

const selectServerConfiguration = `
SELECT ` + QueryMarker + `
	name,
	CONVERT(float, value) AS configured_value,
	CONVERT(float, value_in_use) AS in_use_value
FROM sys.configurations
WHERE name IN (
	N'user connections',
	N'remote admin connections',
	N'network packet size (B)',
	N'remote login timeout (s)',
	N'max server memory (MB)',
	N'min server memory (MB)',
	N'max worker threads',
	N'max degree of parallelism',
	N'cost threshold for parallelism',
	N'optimize for ad hoc workloads',
	N'backup compression default',
	N'min memory per query (KB)',
	N'index create memory (KB)'
)`

const selectAzureSQLDatabaseServerConfiguration = `
SELECT ` + QueryMarker + `
	name,
	CONVERT(float, value) AS configured_value,
	CONVERT(float, value_in_use) AS in_use_value
FROM sys.configurations
WHERE name IN (
	N'network packet size (B)',
	N'remote login timeout (s)',
	N'min memory per query (KB)',
	N'index create memory (KB)'
)`

const selectDatabaseMaxDegreeOfParallelism = `
SELECT ` + QueryMarker + ` CONVERT(float, value)
FROM sys.database_scoped_configurations
WHERE name = N'MAXDOP'`

const selectServerMetrics = `
SELECT ` + QueryMarker + `
	CONVERT(float, si.process_user_time_ms) / 1000.0 AS process_user_cpu_seconds,
	CONVERT(float, si.process_kernel_time_ms) / 1000.0 AS process_kernel_cpu_seconds,
	CONVERT(float, si.ms_ticks - si.sqlserver_start_time_ms_ticks) / 1000.0 AS uptime_seconds,
	CONVERT(float, si.cpu_count) AS logical_cpu_count,
	CONVERT(float, si.committed_kb) * 1024.0 AS memory_manager_committed_bytes,
	CONVERT(float, si.committed_target_kb) * 1024.0 AS memory_manager_target_bytes,
	CONVERT(float, COALESCE(sm.total_physical_memory_kb, 0)) * 1024.0 AS host_physical_memory_total_bytes,
	CONVERT(float, COALESCE(sm.available_physical_memory_kb, 0)) * 1024.0 AS host_physical_memory_available_bytes,
	CONVERT(float, COALESCE(sm.total_page_file_kb, 0)) * 1024.0 AS host_commit_limit_bytes,
	CONVERT(float, COALESCE(sm.available_page_file_kb, 0)) * 1024.0 AS host_commit_available_bytes,
	CONVERT(float, COALESCE(sm.system_high_memory_signal_state, 0)) AS host_high_memory_signal,
	CONVERT(float, COALESCE(sm.system_low_memory_signal_state, 0)) AS host_low_memory_signal,
	CONVERT(float, pm.physical_memory_in_use_kb) * 1024.0 AS process_resident_memory_bytes,
	CONVERT(float, pm.virtual_address_space_committed_kb) * 1024.0 AS process_virtual_memory_bytes,
	CONVERT(float, pm.available_commit_limit_kb) * 1024.0 AS process_available_commit_memory_bytes,
	CONVERT(float, pm.memory_utilization_percentage) / 100.0 AS process_committed_memory_resident_ratio,
	CONVERT(float, pm.page_fault_count) AS process_page_faults,
	CONVERT(float, pm.process_physical_memory_low) AS process_physical_memory_low,
	CONVERT(float, pm.process_virtual_memory_low) AS process_virtual_memory_low
FROM sys.dm_os_sys_info AS si
OUTER APPLY (
	SELECT TOP (1)
		total_physical_memory_kb,
		available_physical_memory_kb,
		total_page_file_kb,
		available_page_file_kb,
		system_high_memory_signal_state,
		system_low_memory_signal_state
	FROM sys.dm_os_sys_memory
) AS sm
CROSS JOIN sys.dm_os_process_memory AS pm`

const selectUserSessions = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'running' THEN 1 ELSE 0 END), 0)) AS running_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'sleeping' THEN 1 ELSE 0 END), 0)) AS sleeping_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'dormant' THEN 1 ELSE 0 END), 0)) AS dormant_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'preconnect' THEN 1 ELSE 0 END), 0)) AS preconnect_sessions,
	CONVERT(int, (SELECT value_in_use FROM sys.configurations WHERE name = N'user connections')) AS configured_session_limit
FROM sys.dm_exec_sessions
WHERE is_user_process = 1
	AND (@exclude_current_user = 0 OR program_name IS NULL OR program_name <> @collector_app_name)`

const selectManagedInstanceMetrics = `
SELECT ` + QueryMarker + `
	CONVERT(float, cpu_rate) / 100.0 AS process_cpu_limit,
	CONVERT(float, process_memory_limit_mb) * 1048576.0 AS process_memory_limit_bytes,
	CONVERT(float, process_memory_limit_mb - non_sos_mem_gap_mb) * 1048576.0 AS sos_memory_target_bytes
FROM sys.dm_os_job_object`

const selectAzureSQLDatabaseResourceMetrics = `
SELECT ` + QueryMarker + ` TOP (1)
	end_time,
	CONVERT(float, avg_cpu_percent) / 100.0 AS database_cpu_utilization_ratio,
	CONVERT(float, avg_memory_usage_percent) / 100.0 AS database_memory_utilization_ratio,
	CONVERT(float, avg_data_io_percent) / 100.0 AS database_data_io_utilization_ratio,
	CONVERT(float, avg_log_write_percent) / 100.0 AS database_log_write_utilization_ratio,
	CONVERT(float, max_worker_percent) / 100.0 AS database_worker_utilization_ratio,
	CONVERT(float, max_session_percent) / 100.0 AS database_session_utilization_ratio,
	CONVERT(float, avg_instance_cpu_percent) / 100.0 AS instance_cpu_utilization_ratio,
	CONVERT(float, avg_instance_memory_percent) / 100.0 AS instance_memory_utilization_ratio
FROM sys.dm_db_resource_stats
ORDER BY end_time DESC`

const selectAzureSQLDatabaseUserSessions = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'running' THEN 1 ELSE 0 END), 0)) AS running_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'sleeping' THEN 1 ELSE 0 END), 0)) AS sleeping_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'dormant' THEN 1 ELSE 0 END), 0)) AS dormant_sessions,
	CONVERT(float, COALESCE(SUM(CASE WHEN status = N'preconnect' THEN 1 ELSE 0 END), 0)) AS preconnect_sessions
FROM sys.dm_exec_sessions
WHERE is_user_process = 1
	AND database_id = DB_ID()
	AND (@exclude_current_user = 0 OR program_name IS NULL OR program_name <> @collector_app_name)`

const selectAzureSQLDatabaseLimits = `
SELECT ` + QueryMarker + `
	CONVERT(float, cpu_limit) AS database_vcore_limit,
	CONVERT(float, dtu_limit) AS database_dtu_limit,
	CONVERT(float, max_sessions) AS user_session_limit,
	CONVERT(float, primary_group_max_workers) AS database_worker_limit,
	CONVERT(float, max_dop) AS database_resource_governor_max_degree_of_parallelism,
	CONVERT(float, max_memory_grant) / 100.0 AS database_memory_grant_limit_ratio
FROM sys.dm_user_db_resource_governance
WHERE database_id = DB_ID()`

const selectManagedInstanceResourceMetrics = `
SELECT ` + QueryMarker + ` TOP (1)
	start_time,
	end_time,
	CONVERT(float, virtual_core_count) AS instance_vcore_limit,
	CONVERT(float, avg_cpu_percent) / 100.0 AS instance_cpu_utilization_ratio,
	CONVERT(float, reserved_storage_mb) * 1048576.0 AS instance_storage_limit_bytes,
	CONVERT(float, storage_space_used_mb) * 1048576.0 AS instance_storage_used_bytes,
	CONVERT(float, io_requests) AS io_requests,
	CONVERT(float, io_bytes_read) AS io_bytes_read,
	CONVERT(float, io_bytes_written) AS io_bytes_written
FROM master.sys.server_resource_stats
ORDER BY start_time DESC`

const selectDatabaseStorage = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(SUM(CASE WHEN type = 0 THEN CONVERT(bigint, FILEPROPERTY(name, 'SpaceUsed')) * 8192 ELSE 0 END), 0)) AS data_storage_used_bytes,
	CONVERT(float, COALESCE(SUM(CASE WHEN type = 0 THEN CONVERT(bigint, size) * 8192 ELSE 0 END), 0)) AS data_storage_allocated_bytes,
	CONVERT(float, COALESCE(SUM(CASE WHEN type = 1 THEN CONVERT(bigint, FILEPROPERTY(name, 'SpaceUsed')) * 8192 ELSE 0 END), 0)) AS log_storage_used_bytes,
	CONVERT(float, COALESCE(SUM(CASE WHEN type = 1 THEN CONVERT(bigint, size) * 8192 ELSE 0 END), 0)) AS log_storage_allocated_bytes,
	CONVERT(float, DATABASEPROPERTYEX(DB_NAME(), 'MaxSizeInBytes')) AS database_storage_limit_bytes
FROM sys.database_files`

const selectVolumeStorage = `
WITH volume_files AS (
	SELECT ` + QueryMarker + `
		vs.volume_mount_point,
		CONVERT(bigint, vfs.size_on_disk_bytes) AS allocated_bytes,
		CONVERT(bigint, vs.total_bytes) AS total_bytes,
		CONVERT(bigint, vs.available_bytes) AS available_bytes,
		ROW_NUMBER() OVER (PARTITION BY vs.volume_mount_point ORDER BY vfs.database_id, vfs.file_id) AS rn
	FROM sys.dm_io_virtual_file_stats(NULL, NULL) AS vfs
	CROSS APPLY sys.dm_os_volume_stats(vfs.database_id, vfs.file_id) AS vs
)
SELECT
	CONVERT(float, SUM(allocated_bytes)) AS instance_storage_allocated_bytes,
	CONVERT(float, SUM(CASE WHEN rn = 1 THEN total_bytes ELSE 0 END)) AS instance_storage_limit_bytes,
	CONVERT(float, SUM(CASE WHEN rn = 1 THEN total_bytes - available_bytes ELSE 0 END)) AS instance_storage_used_bytes,
	CONVERT(float, SUM(CASE WHEN rn = 1 THEN available_bytes ELSE 0 END)) AS instance_storage_available_bytes
FROM volume_files`

const selectPerformanceCounters = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'Batch Requests/sec' THEN cntr_value END), 0)) AS batch_requests_total,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'SQL Compilations/sec' THEN cntr_value END), 0)) AS sql_compilations_total,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'SQL Re-Compilations/sec' THEN cntr_value END), 0)) AS sql_recompilations_total,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'Page life expectancy' AND RTRIM(object_name) LIKE N'%:Buffer Manager' THEN cntr_value END), 0)) AS page_life_expectancy_seconds,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'Memory Grants Pending' THEN cntr_value END), 0)) AS memory_grants_pending,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'Number of Deadlocks/sec' AND RTRIM(instance_name) = N'_Total' THEN cntr_value END), 0)) AS deadlocks_total,
	CONVERT(float, COALESCE(MAX(CASE WHEN RTRIM(counter_name) = N'Lock Waits/sec' AND RTRIM(instance_name) = N'_Total' THEN cntr_value END), 0)) AS lock_waits_total
FROM sys.dm_os_performance_counters
WHERE RTRIM(counter_name) IN (
	N'Batch Requests/sec',
	N'SQL Compilations/sec',
	N'SQL Re-Compilations/sec',
	N'Page life expectancy',
	N'Memory Grants Pending',
	N'Number of Deadlocks/sec',
	N'Lock Waits/sec'
)`

const selectFileIOStats = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(SUM(num_of_bytes_read), 0)) AS io_read_bytes_total,
	CONVERT(float, COALESCE(SUM(num_of_bytes_written), 0)) AS io_written_bytes_total,
	CONVERT(float, COALESCE(SUM(num_of_reads), 0)) AS io_reads_total,
	CONVERT(float, COALESCE(SUM(num_of_writes), 0)) AS io_writes_total,
	CONVERT(float, COALESCE(SUM(io_stall_read_ms), 0)) / 1000.0 AS io_read_stall_seconds_total,
	CONVERT(float, COALESCE(SUM(io_stall_write_ms), 0)) / 1000.0 AS io_write_stall_seconds_total
FROM sys.dm_io_virtual_file_stats(NULL, NULL)`

const selectTempdbSpaceUsage = `
SELECT ` + QueryMarker + `
	CONVERT(float, COALESCE(SUM(user_object_reserved_page_count), 0)) * 8192.0 AS user_objects_bytes,
	CONVERT(float, COALESCE(SUM(internal_object_reserved_page_count), 0)) * 8192.0 AS internal_objects_bytes,
	CONVERT(float, COALESCE(SUM(version_store_reserved_page_count), 0)) * 8192.0 AS version_store_bytes,
	CONVERT(float, COALESCE(SUM(mixed_extent_page_count), 0)) * 8192.0 AS mixed_extents_bytes,
	CONVERT(float, COALESCE(SUM(unallocated_extent_page_count), 0)) * 8192.0 AS unallocated_bytes
FROM tempdb.sys.dm_db_file_space_usage`
