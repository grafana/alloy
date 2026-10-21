package collector

// Define SQL constants and templates here, use the Marker, and add new
// queries to TestQueryMarkers in queries_test.go. For dynamic SQL, test the
// builder output too. This is a review convention, not automatic enforcement.

// AppName identifies collector-owned connections in sys.dm_exec_sessions.program_name.
const AppName = "grafana:alloy:db-o11y"

// QueryMarker identifies collector-owned statements in Query Store and live requests.
// Keep it stable across instances and versions so historical queries remain identifiable.
const QueryMarker = "/* " + AppName + " */"

// pingQuery is the connectivity check this component uses in place of the
// driver's untagged SELECT 1, so liveness probes carry the collector marker.
const PingQuery = "SELECT " + QueryMarker + " 1"

// selectServerInfo returns a stable identifier for the server instance plus
// its product version, used to derive a server_id label and to expose the
// engine version on the connection_info metric.
const SelectServerInfo = `SELECT ` + QueryMarker + `
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ServerName')) AS server_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('MachineName')) AS machine_name,
    CONVERT(NVARCHAR(128), SERVERPROPERTY('ProductVersion')) AS product_version`

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
