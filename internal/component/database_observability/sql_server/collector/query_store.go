package collector

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// queryStoreTarget describes where the collectors read Query Store data from on
// the connected database.
type queryStoreTarget struct {
	database string
	// replicaRolesSupported reports whether Query Store tags runtime statistics
	// with the replica role that recorded them.
	replicaRolesSupported bool
}

// queryRowContexter is satisfied by both *sql.DB and *sql.Conn, so
// queryStoreState can run against either a pooled connection or one pinned
// by a prior USE statement.
type queryRowContexter interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// queryStoreOptions is the row selectQueryStoreState returns for the
// connected database.
type queryStoreOptions struct {
	database              sql.NullString
	actualState           sql.NullString
	captureMode           sql.NullString
	readonlyReason        sql.NullInt64
	replicaRolesSupported bool
}

// usable reports whether the collectors can read Query Store in this state.
func (o queryStoreOptions) usable() bool {
	return strings.EqualFold(strings.TrimSpace(o.actualState.String), "READ_WRITE")
}

// queryStoreState runs selectQueryStoreState against q and returns the row as
// queryStoreOptions. It's the shared primitive behind checkQueryStoreState
// (used by query_metrics/query_details as a preflight) and isQueryStoreEnabled
// (used by health_check) so both agree on what "Query Store state" means
// without duplicating the query and scan.
func queryStoreState(ctx context.Context, q queryRowContexter, queryTimeout time.Duration) (queryStoreOptions, error) {
	var o queryStoreOptions
	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return q.QueryRowContext(queryCtx, selectQueryStoreState).
			Scan(&o.database, &o.actualState, &o.captureMode, &o.readonlyReason, &o.replicaRolesSupported)
	})
	return o, err
}

// checkQueryStoreState reports whether Query Store is readable on the connected
// database and how to read it. It mirrors the preflight used by query_metrics
// so the other Query Store collectors skip cleanly when it's unavailable.
func checkQueryStoreState(ctx context.Context, db *sql.DB, queryTimeout time.Duration, logger *slog.Logger) (queryStoreTarget, bool) {
	opts, err := queryStoreState(ctx, db, queryTimeout)

	if errors.Is(err, sql.ErrNoRows) {
		logger.Warn("Query Store options are unavailable: the login may lack VIEW DATABASE STATE, or the connected database has no Query Store")
		return queryStoreTarget{}, false
	}
	if err != nil {
		logger.Warn("failed to inspect Query Store state; skipping collection", "err", err)
		return queryStoreTarget{}, false
	}

	if !opts.usable() {
		logger.Warn("Query Store is not READ_WRITE; skipping collection",
			"actual_state", opts.actualState.String,
			"capture_mode", opts.captureMode.String,
			"readonly_reason", opts.readonlyReason.Int64)
		return queryStoreTarget{}, false
	}

	return queryStoreTarget{database: opts.database.String, replicaRolesSupported: opts.replicaRolesSupported}, true
}
