package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	HealthCheckCollector = "health_check"
	OP_HEALTH_STATUS     = "health_status"
)

// monitoringUserPrivilegesQuery probes whether the connected user can usefully
// read data from pg_stat_statements.
const monitoringUserPrivilegesQuery = `
	SELECT
		pg_has_role(current_user, 'pg_monitor',        'MEMBER') AS has_pg_monitor_role,
		pg_has_role(current_user, 'pg_read_all_stats', 'MEMBER') AS has_pg_read_all_stats_role,
		has_table_privilege(current_user, 'pg_stat_statements', 'SELECT') AS can_select_pg_stat_statements,
		EXISTS (
			SELECT 1 FROM pg_stat_statements
			WHERE query = '<insufficient privilege>'
		) AS sees_insufficient_privilege`

type HealthCheckArguments struct {
	DB               *sql.DB
	CollectInterval  time.Duration
	ExcludeDatabases []string
	ExcludeUsers     []string
	EntryHandler     loki.EntryHandler

	Logger log.Logger
}

type HealthCheck struct {
	dbConnection     *sql.DB
	collectInterval  time.Duration
	excludeDatabases []string
	excludeUsers     []string
	entryHandler     loki.EntryHandler
	logger           log.Logger

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewHealthCheck(args HealthCheckArguments) (*HealthCheck, error) {
	h := &HealthCheck{
		dbConnection:     args.DB,
		collectInterval:  args.CollectInterval,
		excludeDatabases: args.ExcludeDatabases,
		excludeUsers:     args.ExcludeUsers,
		entryHandler:     args.EntryHandler,
		logger:           log.With(args.Logger, "collector", HealthCheckCollector),
		running:          &atomic.Bool{},
	}
	return h, nil
}

func (c *HealthCheck) Name() string {
	return HealthCheckCollector
}

func (c *HealthCheck) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	c.wg.Go(func() {
		defer c.running.Store(false)

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			c.fetchHealthChecks(c.ctx)
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	})

	return nil
}

func (c *HealthCheck) Stopped() bool {
	return !c.running.Load()
}

func (c *HealthCheck) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

type healthCheckResult struct {
	name   string
	result bool
	value  string
	err    error
}

func (c *HealthCheck) fetchHealthChecks(ctx context.Context) {
	checks := []func(context.Context, *sql.DB) healthCheckResult{
		checkAlloyVersion,
		checkPgStatStatementsEnabled,
		checkTrackActivityQuerySize,
		checkComputeQueryIdEnabled,
		checkMonitoringUserPrivileges,
		c.checkPgStatStatementsHasRows,
	}

	for _, checkFn := range checks {
		result := checkFn(ctx, c.dbConnection)
		if result.err != nil {
			level.Error(c.logger).Log("msg", "health check failed", "check", result.name, "err", result.err)
			continue
		}
		msg := fmt.Sprintf(`check="%s" result="%v" value="%s"`, result.name, result.result, result.value)
		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_HEALTH_STATUS,
			msg,
		)
	}
}

// checkAlloyVersion reports the running Alloy version.
func checkAlloyVersion(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "AlloyVersion"}
	// Always succeeds; returns the version string embedded at build time.
	r.result = true
	r.value = build.Version
	return r
}

// checkPgStatStatementsEnabled verifies that the pg_stat_statements extension is enabled.
func checkPgStatStatementsEnabled(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "PgStatStatementsEnabled"}
	const q = `SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements'`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		r.err = fmt.Errorf("query pg_extension: %w", err)
		return r
	}
	defer rows.Close()

	if rows.Next() {
		r.result = true
	}

	if err := rows.Err(); err != nil {
		r.err = fmt.Errorf("iterate pg_extension: %w", err)
	}

	return r
}

func checkTrackActivityQuerySize(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "TrackActivityQuerySize"}
	const q = `SELECT setting FROM pg_settings WHERE name = 'track_activity_query_size'`
	const expectedSize = 4096

	var sizeString string
	if err := db.QueryRowContext(ctx, q).Scan(&sizeString); err != nil {
		r.err = fmt.Errorf("query track_activity_query_size: %w", err)
		return r
	}

	size, err := strconv.Atoi(sizeString)
	if err != nil {
		r.err = fmt.Errorf("parse track_activity_query_size: %w", err)
		return r
	}

	r.result = size >= expectedSize
	r.value = sizeString

	return r
}

func checkMonitoringUserPrivileges(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "MonitoringUserPrivileges"}

	var hasMonitorRole, hasReadStatsRole, canSelectView, seesInsufficientPrivilege bool
	if err := db.QueryRowContext(ctx, monitoringUserPrivilegesQuery).Scan(
		&hasMonitorRole, &hasReadStatsRole, &canSelectView, &seesInsufficientPrivilege,
	); err != nil {
		r.err = fmt.Errorf("query monitoring user privileges: %w", err)
		return r
	}

	// The 'result' doesn't take role membership into account, but we report the
	// role checks in 'value' for diagnostics:
	// checks in 'value' for diagnostics:
	//
	// - canSelectView (i.e. can read the view) and !seesInsufficientPrivilege (i.e.
	//   no '<insufficient privilege>' rows visible) gate result, because
	//   together they prove the user can actually read real data
	//   regardless of how that access was granted.
	//
	// - hasMonitorRole / hasReadStatsRole are reported in value for diagnostics only.
	//   pg_has_role(_, _, 'MEMBER') resolves transitive membership, so
	//   granting pg_monitor implies has_pg_read_all_stats_role=true (pg_monitor is
	//   its parent role); the reverse does not hold. Role membership is also
	//   not the only path: a direct GRANT SELECT on pg_stat_statements lets a
	//   user read the view without either role - they show can_select_view=true but
	//   will see '<insufficient privilege>' for queries from other users,
	//   which sees_insufficient_privilege catches. Treating the role columns as gating
	//   would produce false negatives on perfectly valid setups.
	//
	// Example: a user with only `GRANT SELECT ON pg_stat_statements TO ...`
	// reports can_select_view=true, has_pg_monitor_role=false, has_pg_read_all_stats_role=false,
	// sees_insufficient_privilege=true once any other user has executed a
	// statement; result is correctly false because the collector won't get
	// useful data, even though the role checks alone wouldn't have told us
	// whether SELECT was reachable.
	r.result = canSelectView && !seesInsufficientPrivilege
	r.value = fmt.Sprintf(
		"can_select_view=%v,has_pg_monitor_role=%v,has_pg_read_all_stats_role=%v,sees_insufficient_privilege=%v",
		canSelectView, hasMonitorRole, hasReadStatsRole, seesInsufficientPrivilege,
	)
	return r
}

// checkComputeQueryIdEnabled verifies the compute_query_id is not disabled
func checkComputeQueryIdEnabled(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "ComputeQueryIdEnabled"}
	const q = `SELECT setting FROM pg_settings WHERE name = 'compute_query_id'`

	var setting string
	if err := db.QueryRowContext(ctx, q).Scan(&setting); err != nil {
		r.err = fmt.Errorf("query compute_query_id: %w", err)
		return r
	}
	r.value = setting
	r.result = setting != "off"
	return r
}

// checkPgStatStatementsHasRows ensures pg_stat_statements has at least one row
// with a real queryid (non-null and non-zero) for a database/user we'd actually
// ingest after applying the exclude_databases / exclude_users filters.
func (c *HealthCheck) checkPgStatStatementsHasRows(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "PgStatStatementsHasRows"}
	excludedDatabasesClause := buildExcludedDatabasesClause(c.excludeDatabases)
	excludedUsersClause := buildExcludedUsersClause(c.excludeUsers, "pg_get_userbyid(pg_stat_statements.userid)")
	q := fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_stat_statements
			JOIN pg_database ON pg_database.oid = pg_stat_statements.dbid
			WHERE pg_database.datname NOT IN %s
			  AND pg_stat_statements.queryid <> 0
			  %s
		)`, excludedDatabasesClause, excludedUsersClause)

	var hasRows bool
	if err := db.QueryRowContext(ctx, q).Scan(&hasRows); err != nil {
		r.err = err
		return r
	}
	r.result = hasRows
	return r
}
