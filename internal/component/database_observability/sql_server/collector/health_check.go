package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	HealthCheckCollector = "health_check"
)

// requiredDatabasePermissions are the database-scoped permissions the
// monitoring login must hold on every non-excluded database, per the grants
// provisioned by deployment_tools' setup/sqlserver.libsonnet: VIEW DEFINITION
// (schema introspection) and VIEW DATABASE STATE (Query Store).
var requiredDatabasePermissions = []string{"VIEW DEFINITION", "VIEW DATABASE STATE"}

const selectMyPermissionsQuery = `SELECT permission_name FROM sys.fn_my_permissions(NULL, 'DATABASE')`

const selectQueryStoreHasRowsQuery = `SELECT TOP 1 1 FROM sys.query_store_runtime_stats`

const noAccessibleDatabasesValue = "no accessible databases"

type HealthCheckArguments struct {
	DB               *sql.DB
	CollectInterval  time.Duration
	QueryTimeout     time.Duration
	ExcludeDatabases []string
	EntryHandler     loki.EntryHandler

	Logger *slog.Logger
}

type HealthCheck struct {
	dbConnection     *sql.DB
	collectInterval  time.Duration
	queryTimeout     time.Duration
	excludeDatabases []string
	entryHandler     loki.EntryHandler
	logger           *slog.Logger

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewHealthCheck(args HealthCheckArguments) (*HealthCheck, error) {
	h := &HealthCheck{
		dbConnection:     args.DB,
		collectInterval:  args.CollectInterval,
		queryTimeout:     queryTimeoutOrDefault(args.QueryTimeout),
		excludeDatabases: args.ExcludeDatabases,
		entryHandler:     args.EntryHandler,
		logger:           args.Logger.With("collector", HealthCheckCollector),
		running:          &atomic.Bool{},
	}
	return h, nil
}

func (c *HealthCheck) Name() string {
	return HealthCheckCollector
}

func (c *HealthCheck) Start(ctx context.Context) error {
	c.logger.Debug("collector started")

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
	results, err := c.runPerDatabaseChecks(ctx)
	if err != nil {
		c.logger.Error("health check failed", "check", "per-database checks", "err", err)
	} else {
		for _, result := range results {
			c.emit(result)
		}
	}

	c.emit(checkAlloyVersion())
}

func (c *HealthCheck) emit(result healthCheckResult) {
	if result.err != nil {
		c.logger.Error("health check failed", "check", result.name, "err", result.err)
		return
	}
	msg := fmt.Sprintf(`check="%s" result="%v" value="%s"`, result.name, result.result, result.value)
	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		database_observability.OP_HEALTH_STATUS,
		msg,
	)
}

// checkAlloyVersion reports the running Alloy version.
func checkAlloyVersion() healthCheckResult {
	return healthCheckResult{name: "AlloyVersion", result: true, value: build.Version}
}

// runPerDatabaseChecks lists the non-excluded, accessible databases on the
// instance and, for each one, checks the monitoring login's grants and the
// database's Query Store state in a single pass (one USE per database).
// It returns the aggregate RequiredGrantsPresent, QueryStoreEnabled, and
// QueryStoreHasRows results.
//
// RequiredGrantsPresent and QueryStoreEnabled are setup-correctness checks:
// they must hold on every non-excluded database to pass, mirroring
// mysql/postgres's "missing grants: X, Y" diagnostic idiom by listing any
// non-compliant databases in the value field.
//
// QueryStoreHasRows is a traffic/data check, not a setup check: it mirrors
// mysql's PerformanceSchemaHasRows / postgres's PgStatStatementsHasRows,
// which pass as soon as the introspection mechanism has any row anywhere
// across excluded schemas/databases. Requiring every database to have rows
// would fail the whole health check for a legitimately low-traffic database,
// so this check passes if any non-excluded database has rows.
func (c *HealthCheck) runPerDatabaseChecks(ctx context.Context) ([]healthCheckResult, error) {
	databases, err := listDatabases(ctx, c.dbConnection, c.queryTimeout, c.excludeDatabases)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	grantsResult := healthCheckResult{name: "RequiredGrantsPresent", result: true}
	queryStoreEnabledResult := healthCheckResult{name: "QueryStoreEnabled", result: true}
	queryStoreHasRowsResult := healthCheckResult{name: "QueryStoreHasRows", result: false}

	if len(databases) == 0 {
		grantsResult.value = noAccessibleDatabasesValue
		queryStoreEnabledResult.value = noAccessibleDatabasesValue
		queryStoreHasRowsResult.value = noAccessibleDatabasesValue
		return []healthCheckResult{grantsResult, queryStoreEnabledResult, queryStoreHasRowsResult}, nil
	}

	var conn *sql.Conn
	if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
		var err error
		conn, err = c.dbConnection.Conn(queryCtx)
		return err
	}); err != nil {
		return nil, fmt.Errorf("failed to acquire database connection: %w", err)
	}
	defer conn.Close()

	var missingGrants, queryStoreDisabled, skippedDatabases []string
	hasRows := false

	for _, db := range databases {
		if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
			_, err := conn.ExecContext(queryCtx, fmt.Sprintf("USE %s", db.quoted))
			return err
		}); err != nil {
			// A failed USE is a check-execution error (e.g. the database is
			// briefly offline or mid-restore), not evidence of missing grants
			// or a disabled Query Store — don't let it masquerade as either.
			c.logger.Error("failed to switch database context, skipping", "database", db.name, "err", err)
			skippedDatabases = append(skippedDatabases, db.name)
			continue
		}

		if ok, err := hasRequiredPermissions(ctx, conn, c.queryTimeout); err != nil {
			c.logger.Error("failed to check permissions", "database", db.name, "err", err)
			missingGrants = append(missingGrants, db.name)
		} else if !ok {
			missingGrants = append(missingGrants, db.name)
		}

		enabled, err := isQueryStoreEnabled(ctx, conn, c.queryTimeout)
		if err != nil {
			c.logger.Error("failed to check Query Store state", "database", db.name, "err", err)
			queryStoreDisabled = append(queryStoreDisabled, db.name)
		} else if !enabled {
			queryStoreDisabled = append(queryStoreDisabled, db.name)
		}

		if !hasRows {
			rows, err := queryStoreHasRows(ctx, conn, c.queryTimeout)
			if err != nil {
				c.logger.Error("failed to check Query Store rows", "database", db.name, "err", err)
			} else if rows {
				hasRows = true
			}
		}
	}

	if len(missingGrants) > 0 {
		sort.Strings(missingGrants)
		grantsResult.result = false
		grantsResult.value = fmt.Sprintf("missing grants on: %s", strings.Join(missingGrants, ", "))
	}

	if len(queryStoreDisabled) > 0 {
		sort.Strings(queryStoreDisabled)
		queryStoreEnabledResult.result = false
		queryStoreEnabledResult.value = fmt.Sprintf("query store not enabled on: %s", strings.Join(queryStoreDisabled, ", "))
	}

	queryStoreHasRowsResult.result = hasRows

	if len(skippedDatabases) > 0 {
		sort.Strings(skippedDatabases)
		skippedNote := fmt.Sprintf(" (could not check: %s)", strings.Join(skippedDatabases, ", "))
		grantsResult.value += skippedNote
		queryStoreEnabledResult.value += skippedNote
		queryStoreHasRowsResult.value += skippedNote
	}

	return []healthCheckResult{grantsResult, queryStoreEnabledResult, queryStoreHasRowsResult}, nil
}

// hasRequiredPermissions checks the requiredDatabasePermissions against the
// database currently active on conn (set via a prior USE statement).
func hasRequiredPermissions(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	granted := map[string]bool{}

	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		rows, err := conn.QueryContext(queryCtx, selectMyPermissionsQuery)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var permission string
			if err := rows.Scan(&permission); err != nil {
				return fmt.Errorf("scan permission_name: %w", err)
			}
			granted[strings.ToUpper(permission)] = true
		}
		return rows.Err()
	})
	if err != nil {
		return false, err
	}

	for _, required := range requiredDatabasePermissions {
		if !granted[required] {
			return false, nil
		}
	}
	return true, nil
}

// isQueryStoreEnabled checks whether Query Store is usable (READ_WRITE) on
// the database currently active on conn (set via a prior USE statement).
// It mirrors the "usable" definition used by the query_metrics preflight in
// checkQueryStoreState (query_store.go), rather than the raw ON/OFF state.
func isQueryStoreEnabled(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	var database, actualState, captureMode sql.NullString
	var readonlyReason sql.NullInt64

	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return conn.QueryRowContext(queryCtx, selectQueryStoreState).
			Scan(&database, &actualState, &captureMode, &readonlyReason)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(actualState.String), "READ_WRITE"), nil
}

// queryStoreHasRows checks whether sys.query_store_runtime_stats has any row
// on the database currently active on conn (set via a prior USE statement).
func queryStoreHasRows(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	var found int
	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		return conn.QueryRowContext(queryCtx, selectQueryStoreHasRowsQuery).Scan(&found)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
