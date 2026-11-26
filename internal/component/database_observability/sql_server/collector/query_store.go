package collector

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// Query Store actual_state_desc values the collectors can read from.
const (
	queryStoreStateReadWrite = "READ_WRITE"
	// queryStoreStateReadCaptureSecondary is reported on a readable secondary
	// replica when Query Store for secondary replicas is active: the replica's
	// executions are sent to the primary's Query Store and replicated back.
	queryStoreStateReadCaptureSecondary = "READ_CAPTURE_SECONDARY"
)

// Replica roles as reported by sys.query_store_replicas.role_type. Azure SQL
// Database reports role 3 with replica_name "Geo Secondary" and records a
// geo-secondary's executions under it, although the sys.query_store_replicas
// reference page labels 3 as Geo-Primary and 4 as Geo-Secondary. Role 4 is
// "Geo HA Secondary", a high-availability replica of a geo-secondary.
const (
	replicaRolePrimary      = 1
	replicaRoleSecondary    = 2
	replicaRoleGeoSecondary = 3
)

// Azure SQL Database Hyperscale replica roles as reported by
// sys.dm_hs_database_replicas.replica_role. These differ from the Query Store
// role_type values above.
const (
	hyperscaleRolePrimary      = 0
	hyperscaleRoleHASecondary  = 1
	hyperscaleRoleGeoForwarder = 2
)

const hyperscaleEdition = "Hyperscale"

type queryStoreMode int

const (
	// queryStoreModePrimary reads the executions recorded by the primary role.
	queryStoreModePrimary queryStoreMode = iota + 1
	// queryStoreModeSecondary reads the executions recorded by the connected
	// secondary replica's role.
	queryStoreModeSecondary
)

// queryStoreTarget describes where the collectors read Query Store data from on
// the connected database.
type queryStoreTarget struct {
	database string
	mode     queryStoreMode
	// roleType is the sys.query_store_replicas role_type whose executions the
	// connected server recorded.
	roleType int
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
	return isReadableQueryStoreState(o.actualState.String)
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

// isReadableQueryStoreState reports whether actualState is a Query Store state
// the collectors can read the connected server's executions from.
func isReadableQueryStoreState(actualState string) bool {
	switch strings.ToUpper(strings.TrimSpace(actualState)) {
	case queryStoreStateReadWrite, queryStoreStateReadCaptureSecondary:
		return true
	default:
		return false
	}
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

	switch state := strings.ToUpper(strings.TrimSpace(opts.actualState.String)); state {
	case queryStoreStateReadWrite:
		return queryStoreTarget{database: opts.database.String, mode: queryStoreModePrimary, roleType: replicaRolePrimary, replicaRolesSupported: opts.replicaRolesSupported}, true
	case queryStoreStateReadCaptureSecondary:
		roleType, err := secondaryReplicaRole(ctx, db, queryTimeout)
		if err != nil {
			logger.Warn("failed to identify the secondary replica role; skipping collection", "err", err)
			return queryStoreTarget{}, false
		}
		if roleType == 0 {
			logger.Warn("Query Store records secondary replica executions, but the replica role can't be identified on this edition; skipping collection",
				"actual_state", opts.actualState.String)
			return queryStoreTarget{}, false
		}
		return queryStoreTarget{database: opts.database.String, mode: queryStoreModeSecondary, roleType: roleType, replicaRolesSupported: opts.replicaRolesSupported}, true
	default:
		logger.Warn("Query Store is not readable; skipping collection",
			"actual_state", opts.actualState.String,
			"capture_mode", opts.captureMode.String,
			"readonly_reason", opts.readonlyReason.Int64)
		return queryStoreTarget{}, false
	}
}

// secondaryReplicaRole returns the sys.query_store_replicas role_type of the
// connected read-only secondary, or 0 when it can't be identified. Only Azure
// SQL Database is supported: a geo-replication link in the SECONDARY role means
// a geo-secondary, otherwise it's a high-availability read replica. The
// availability group views aren't used because some read-only replicas, such
// as Amazon RDS for SQL Server read replicas, report themselves as the primary
// of their own availability group.
//
// Hyperscale roles are read from sys.dm_hs_database_replicas instead, because
// a Hyperscale named replica has no geo-replication link yet records its
// executions under its own Query Store role, which can't be resolved to a
// role_type. Named replicas, and high-availability replicas of a named replica
// or a geo-secondary, return 0.
func secondaryReplicaRole(ctx context.Context, db *sql.DB, queryTimeout time.Duration) (int, error) {
	var engineEdition int
	var updateability, edition sql.NullString
	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return db.QueryRowContext(queryCtx, selectDatabaseReplicaProperties).Scan(&engineEdition, &updateability, &edition)
	})
	if err != nil {
		return 0, err
	}
	if engineEdition != azureSQLDatabaseEngineEdition || !strings.EqualFold(updateability.String, "READ_ONLY") {
		return 0, nil
	}
	if strings.EqualFold(edition.String, hyperscaleEdition) {
		return hyperscaleReplicaRole(ctx, db, queryTimeout)
	}

	var geoSecondaryLinks int
	err = withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return db.QueryRowContext(queryCtx, selectGeoSecondaryLinks).Scan(&geoSecondaryLinks)
	})
	if err != nil {
		return 0, err
	}
	if geoSecondaryLinks > 0 {
		return replicaRoleGeoSecondary, nil
	}
	return replicaRoleSecondary, nil
}

// hyperscaleReplicaRole returns the sys.query_store_replicas role_type of the
// connected Hyperscale replica, or 0 when it's neither a high-availability
// replica of the primary nor a geo-secondary.
func hyperscaleReplicaRole(ctx context.Context, db *sql.DB, queryTimeout time.Duration) (int, error) {
	var role int
	var parentRole sql.NullInt64
	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return db.QueryRowContext(queryCtx, selectHyperscaleReplicaRole).Scan(&role, &parentRole)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	switch {
	case role == hyperscaleRoleHASecondary && parentRole.Valid && parentRole.Int64 == hyperscaleRolePrimary:
		return replicaRoleSecondary, nil
	case role == hyperscaleRoleGeoForwarder:
		return replicaRoleGeoSecondary, nil
	default:
		return 0, nil
	}
}
