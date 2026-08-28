package collector

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
)

// selectQueryStoreState reads the connected database's Query Store state
const selectQueryStoreState = `
	SELECT
		DB_NAME(),
		actual_state_desc,
		query_capture_mode_desc,
		readonly_reason
	FROM sys.database_query_store_options`

// checkQueryStoreState reports whether Query Store is readable on the connected
// database and returns that database's name. It mirrors the preflight used by
// query_metrics so query_details skips cleanly when Query Store is unavailable.
func checkQueryStoreState(ctx context.Context, db *sql.DB, logger *slog.Logger) (string, bool) {
	var database, actualState, captureMode sql.NullString
	var readonlyReason sql.NullInt64

	err := db.QueryRowContext(ctx, selectQueryStoreState).
		Scan(&database, &actualState, &captureMode, &readonlyReason)

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
