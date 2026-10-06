package collector

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// selectQueryStoreState reads the connected database's Query Store state
const selectQueryStoreState = `
	SELECT
		DB_NAME(),
		actual_state_desc,
		query_capture_mode_desc,
		readonly_reason
	FROM sys.database_query_store_options`

// queryRowContexter is satisfied by both *sql.DB and *sql.Conn, so
// queryStoreState can run against either a pooled connection or one pinned
// by a prior USE statement.
type queryRowContexter interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// queryStoreState runs selectQueryStoreState against q and returns its raw
// fields. It's the shared primitive behind checkQueryStoreState (used by
// query_metrics/query_details as a preflight) and isQueryStoreEnabled (used
// by health_check) so both agree on what "Query Store state" means without
// duplicating the query and scan.
func queryStoreState(ctx context.Context, q queryRowContexter, queryTimeout time.Duration) (database, actualState, captureMode sql.NullString, readonlyReason sql.NullInt64, err error) {
	err = withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return q.QueryRowContext(queryCtx, selectQueryStoreState).
			Scan(&database, &actualState, &captureMode, &readonlyReason)
	})
	return database, actualState, captureMode, readonlyReason, err
}

// checkQueryStoreState reports whether Query Store is readable on the connected
// database and returns that database's name. It mirrors the preflight used by
// query_metrics so query_details skips cleanly when Query Store is unavailable.
func checkQueryStoreState(ctx context.Context, db *sql.DB, queryTimeout time.Duration, logger *slog.Logger) (string, bool) {
	database, actualState, captureMode, readonlyReason, err := queryStoreState(ctx, db, queryTimeout)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Warn("Query Store options are unavailable: the login may lack VIEW DATABASE STATE, or the connected database has no Query Store")
		return "", false
	}
	if err != nil {
		logger.Warn("failed to inspect Query Store state; skipping collection", "err", err)
		return "", false
	}

	if state := strings.ToUpper(strings.TrimSpace(actualState.String)); state != "READ_WRITE" {
		logger.Warn("Query Store is not READ_WRITE; skipping collection",
			"actual_state", actualState.String,
			"capture_mode", captureMode.String,
			"readonly_reason", readonlyReason.Int64)
		return "", false
	}

	return database.String, true
}
