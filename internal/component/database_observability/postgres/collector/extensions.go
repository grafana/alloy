package collector

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// DetectedExtensions records which Postgres extensions of interest are installed
// on the target database. It is populated once at collector startup and passed
// into collectors that need to adapt their behavior to the extension's presence.
type DetectedExtensions struct {
	TimescaleDB bool
}

// timescaleDBInternalSchemas lists the schemas that hold TimescaleDB's private
// catalog and runtime objects. Queries against these schemas are emitted by
// TimescaleDB's bgworker and SPI invocations and frequently contain bare
// identifiers or multi-branch type-mismatched UNIONs that fail under standalone
// PREPARE/EXPLAIN.
var timescaleDBInternalSchemas = []string{
	"_timescaledb_cache",
	"_timescaledb_catalog",
	"_timescaledb_config",
	"_timescaledb_debug",
	"_timescaledb_functions",
	"_timescaledb_internal",
}

// timescaleDBMetadataSchemas lists the public-facing TimescaleDB schemas that
// hold metadata views (hypertables, chunks, jobs, etc.) rather than user data.
// They are excluded from schema_details by default but not from explain_plans,
// since users sometimes write queries against them directly.
var timescaleDBMetadataSchemas = []string{
	"timescaledb_experimental",
	"timescaledb_information",
	"toolkit_experimental",
}

// timescaleDBExplainSkipRegex is the POSIX alternation matched against
// pg_stat_statements.query to exclude statements that touch TimescaleDB
// internal schemas.
const timescaleDBExplainSkipRegex = `_timescaledb_(cache|catalog|config|debug|functions|internal)\.`

const selectDatabasesForExtensionDetection = `
SELECT datname
FROM pg_database
WHERE datistemplate = false
	AND has_database_privilege(datname, 'CONNECT')
	AND datname NOT IN %s`

// DetectExtensions probes pg_extension for known extensions and returns the
// resulting flags. Failures (including permission errors against pg_extension)
// are logged and fall back to probing known extension-owned schemas so
// permission-restricted users can still get extension-aware filtering.
func DetectExtensions(ctx context.Context, db *sql.DB, logger log.Logger) DetectedExtensions {
	const q = `SELECT extname FROM pg_extension WHERE extname IN ('timescaledb')`
	var det DetectedExtensions
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		level.Warn(logger).Log("msg", "failed to detect postgres extensions from pg_extension; falling back to schema probe", "err", err)
		return detectExtensionsFromSchemas(ctx, db, logger)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			level.Warn(logger).Log("msg", "failed to scan extension row; falling back to schema probe", "err", err)
			return detectExtensionsFromSchemas(ctx, db, logger)
		}
		switch name {
		case "timescaledb":
			det.TimescaleDB = true
		}
	}
	if err := rows.Err(); err != nil {
		level.Warn(logger).Log("msg", "error iterating extension rows; falling back to schema probe", "err", err)
		return detectExtensionsFromSchemas(ctx, db, logger)
	}
	if det.TimescaleDB {
		level.Info(logger).Log("msg", "detected postgres extension", "extension", "timescaledb")
	}
	return det
}

func detectExtensionsFromSchemas(ctx context.Context, db *sql.DB, logger log.Logger) DetectedExtensions {
	var det DetectedExtensions
	q := fmt.Sprintf(
		`SELECT nspname FROM pg_namespace WHERE nspname IN %s`,
		database_observability.BuildExclusionClause(timescaleDBInternalSchemas),
	)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		level.Warn(logger).Log("msg", "failed to detect postgres extensions from schema probe; falling back to default behavior", "err", err)
		return det
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			level.Warn(logger).Log("msg", "failed to scan extension schema row", "err", err)
			return det
		}
		for _, schema := range timescaleDBInternalSchemas {
			if name == schema {
				det.TimescaleDB = true
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		level.Warn(logger).Log("msg", "error iterating extension schema rows", "err", err)
		return det
	}
	if det.TimescaleDB {
		level.Info(logger).Log("msg", "detected postgres extension from schema probe", "extension", "timescaledb")
	}
	return det
}

// DetectExtensionsAcrossDatabases probes each connectable database because
// Postgres extensions are installed per database while pg_stat_statements is
// queried across databases by the explain_plans collector.
func DetectExtensionsAcrossDatabases(ctx context.Context, db *sql.DB, dsn string, excludeDatabases []string, factory databaseConnectionFactory, logger log.Logger) DetectedExtensions {
	if factory == nil {
		factory = defaultDbConnectionFactory
	}

	databases, err := databasesForExtensionDetection(ctx, db, excludeDatabases)
	if err != nil {
		level.Warn(logger).Log("msg", "failed to list databases for extension detection; falling back to initial database", "err", err)
		return DetectExtensions(ctx, db, logger)
	}
	if len(databases) == 0 {
		return DetectExtensions(ctx, db, logger)
	}

	var combined DetectedExtensions
	probed := false
	for _, database := range databases {
		databaseDSN, err := replaceDatabaseNameInDSN(dsn, database)
		if err != nil {
			level.Warn(logger).Log("msg", "failed to build database-specific DSN for extension detection", "database", database, "err", err)
			continue
		}
		databaseConnection, err := factory(databaseDSN)
		if err != nil {
			level.Warn(logger).Log("msg", "failed to connect to database for extension detection", "database", database, "err", err)
			continue
		}
		probed = true
		det := DetectExtensions(ctx, databaseConnection, logger)
		combined.TimescaleDB = combined.TimescaleDB || det.TimescaleDB
		if databaseConnection != db {
			if err := databaseConnection.Close(); err != nil {
				level.Warn(logger).Log("msg", "failed to close extension detection database connection", "database", database, "err", err)
			}
		}
	}
	if !probed {
		return DetectExtensions(ctx, db, logger)
	}
	return combined
}

func databasesForExtensionDetection(ctx context.Context, db *sql.DB, excludeDatabases []string) ([]string, error) {
	query := fmt.Sprintf(selectDatabasesForExtensionDetection, buildExcludedDatabasesClause(excludeDatabases))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var datname string
		if err := rows.Scan(&datname); err != nil {
			return nil, err
		}
		databases = append(databases, datname)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return databases, nil
}

// TimescaleDBInternalSchemas returns the schemas that hold TimescaleDB's
// private catalog and runtime objects. Returned as a fresh slice so callers
// may safely append.
func TimescaleDBInternalSchemas() []string {
	out := make([]string, len(timescaleDBInternalSchemas))
	copy(out, timescaleDBInternalSchemas)
	return out
}

// TimescaleDBMetadataSchemas returns the public TimescaleDB metadata schemas.
// Returned as a fresh slice so callers may safely append.
func TimescaleDBMetadataSchemas() []string {
	out := make([]string, len(timescaleDBMetadataSchemas))
	copy(out, timescaleDBMetadataSchemas)
	return out
}

// TimescaleDBExplainSkipRegex returns the POSIX alternation used to filter
// TimescaleDB-internal queries out of pg_stat_statements before EXPLAIN.
func TimescaleDBExplainSkipRegex() string {
	return timescaleDBExplainSkipRegex
}

// buildExtensionExplainFilterClause produces the SQL fragment appended to
// selectQueriesForExplainPlanTemplate when extension filtering is active.
// Empty string means no filter.
func buildExtensionExplainFilterClause(skip bool, det DetectedExtensions) string {
	if !skip || !det.TimescaleDB {
		return ""
	}
	return fmt.Sprintf("AND s.query !~ '%s'", TimescaleDBExplainSkipRegex())
}
