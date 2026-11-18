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

// requiredSchemaPermission is the database-scoped permission the monitoring
// login must hold on every non-excluded database for schema introspection.
const requiredSchemaPermission = "VIEW DEFINITION"

// queryStoreAccessPermissions are the database-scoped permissions that grant
// Query Store access; either satisfies the requirement. VIEW DATABASE STATE
// is required on SQL Server 2019 and earlier; VIEW DATABASE PERFORMANCE
// STATE is the SQL Server 2022+ least-privilege equivalent and is reported
// as a distinct permission name by sys.fn_my_permissions.
var queryStoreAccessPermissions = []string{"VIEW DATABASE STATE", "VIEW DATABASE PERFORMANCE STATE"}

// serverStatePermissions are the server-scoped permissions that grant access to
// server-level DMVs used by server_metrics and query_samples; either satisfies
// the requirement on editions other than Azure SQL Database.
var serverStatePermissions = []string{"VIEW SERVER STATE", "VIEW SERVER PERFORMANCE STATE"}

const (
	noAccessibleDatabasesValue = "no accessible databases"
	missingServerGrantValue    = "missing server grant: VIEW SERVER STATE or VIEW SERVER PERFORMANCE STATE"
)

type HealthCheckArguments struct {
	DB               *sql.DB
	CollectInterval  time.Duration
	QueryTimeout     time.Duration
	EngineEdition    int
	ExcludeDatabases []string
	IncludeDatabases []string
	EntryHandler     loki.EntryHandler

	Logger *slog.Logger
}

type HealthCheck struct {
	dbConnection     *sql.DB
	collectInterval  time.Duration
	queryTimeout     time.Duration
	engineEdition    int
	excludeDatabases []string
	includeDatabases []string
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
		engineEdition:    args.EngineEdition,
		excludeDatabases: args.ExcludeDatabases,
		includeDatabases: args.IncludeDatabases,
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
	databases, err := listDatabases(ctx, c.dbConnection, c.queryTimeout, c.excludeDatabases, c.includeDatabases)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	grantsResult := healthCheckResult{name: "RequiredGrantsPresent", result: true}
	queryStoreEnabledResult := healthCheckResult{name: "QueryStoreEnabled", result: true}
	queryStoreHasRowsResult := healthCheckResult{name: "QueryStoreHasRows", result: false}

	if len(databases) == 0 {
		grantsResult.result = false
		grantsResult.value = noAccessibleDatabasesValue
		queryStoreEnabledResult.result = false
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

	var serverGrantMissing bool
	var serverPermErr error
	if c.engineEdition != azureSQLDatabaseEngineEdition {
		ok, err := hasRequiredServerPermissions(ctx, conn, c.queryTimeout)
		if err != nil {
			c.logger.Error("failed to check server permissions", "err", err)
			serverPermErr = err
		} else if !ok {
			serverGrantMissing = true
		}
	}

	var missingGrants, queryStoreDisabled, skippedDatabases []string
	var grantsCheckSkipped, queryStoreCheckSkipped []string
	var grantsEvaluatedCount, queryStoreStateEvaluatedCount, queryStoreRowsEvaluatedCount int
	hasRows := false

	for _, db := range databases {
		if err := withQueryTimeout(ctx, c.queryTimeout, func(queryCtx context.Context) error {
			_, err := conn.ExecContext(queryCtx, fmt.Sprintf(useDatabaseTemplate, db.quoted))
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
			// A query error here (e.g. a timeout) means we couldn't evaluate
			// the grant, not that the grant is missing — don't conflate the
			// two, or a transient failure reads as a persistent setup problem.
			c.logger.Error("failed to check permissions", "database", db.name, "err", err)
			grantsCheckSkipped = append(grantsCheckSkipped, db.name)
		} else {
			grantsEvaluatedCount++
			if !ok {
				missingGrants = append(missingGrants, db.name)
			}
		}

		enabled, err := isQueryStoreEnabled(ctx, conn, c.queryTimeout)
		if err != nil {
			c.logger.Error("failed to check Query Store state", "database", db.name, "err", err)
			queryStoreCheckSkipped = append(queryStoreCheckSkipped, db.name)
		} else {
			queryStoreStateEvaluatedCount++
			if !enabled {
				queryStoreDisabled = append(queryStoreDisabled, db.name)
			}
		}

		if !hasRows {
			rows, err := queryStoreHasRows(ctx, conn, c.queryTimeout)
			if err != nil {
				c.logger.Error("failed to check Query Store rows", "database", db.name, "err", err)
			} else {
				queryStoreRowsEvaluatedCount++
				if rows {
					hasRows = true
				}
			}
		}
	}

	var grantFailureReasons []string
	if serverGrantMissing {
		grantFailureReasons = append(grantFailureReasons, missingServerGrantValue)
	}
	if v := formatDatabaseList("missing grants on: ", "", missingGrants); v != "" {
		grantFailureReasons = append(grantFailureReasons, v)
	}
	if len(grantFailureReasons) > 0 {
		grantsResult.result = false
		grantsResult.value = strings.Join(grantFailureReasons, "; ")
		if serverPermErr != nil {
			grantsCheckSkipped = append(grantsCheckSkipped, "server")
		}
	} else if serverPermErr != nil {
		grantsResult.err = fmt.Errorf("failed to check server permissions: %w", serverPermErr)
	} else if grantsEvaluatedCount == 0 {
		grantsResult.err = errors.New("could not check permissions on any database")
	}

	if v := formatDatabaseList("query store not enabled on: ", "", queryStoreDisabled); v != "" {
		queryStoreEnabledResult.result = false
		queryStoreEnabledResult.value = v
	} else if queryStoreStateEvaluatedCount == 0 {
		queryStoreEnabledResult.err = errors.New("could not check Query Store state on any database")
	}

	queryStoreHasRowsResult.result = hasRows
	if !hasRows && queryStoreRowsEvaluatedCount == 0 {
		queryStoreHasRowsResult.err = errors.New("could not check Query Store rows on any database")
	}

	if note := formatDatabaseList(" (could not check: ", ")", skippedDatabases, grantsCheckSkipped); note != "" {
		grantsResult.value += note
	}
	if note := formatDatabaseList(" (could not check: ", ")", skippedDatabases, queryStoreCheckSkipped); note != "" {
		queryStoreEnabledResult.value += note
	}
	if note := formatDatabaseList(" (could not check: ", ")", skippedDatabases); note != "" {
		queryStoreHasRowsResult.value += note
	}

	return []healthCheckResult{grantsResult, queryStoreEnabledResult, queryStoreHasRowsResult}, nil
}

// formatDatabaseList dedupes and sorts database names across all of lists,
// joins them, and wraps the result in prefix/suffix. Returns "" if every
// list is empty, so callers can use that to decide whether to apply the
// result at all (e.g. leaving a result's default value/status untouched).
func formatDatabaseList(prefix, suffix string, lists ...[]string) string {
	unique := map[string]struct{}{}
	var deduped []string
	for _, databases := range lists {
		for _, db := range databases {
			if _, ok := unique[db]; !ok {
				unique[db] = struct{}{}
				deduped = append(deduped, db)
			}
		}
	}
	if len(deduped) == 0 {
		return ""
	}
	sort.Strings(deduped)
	return prefix + strings.Join(deduped, ", ") + suffix
}

// hasRequiredServerPermissions checks that the login has at least one of
// serverStatePermissions at the server level.
func hasRequiredServerPermissions(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	granted, err := queryGrantedPermissions(ctx, conn, queryTimeout, selectMyServerPermissionsQuery)
	if err != nil {
		return false, err
	}

	for _, alternative := range serverStatePermissions {
		if granted[alternative] {
			return true, nil
		}
	}
	return false, nil
}

// hasRequiredPermissions checks that the login has requiredSchemaPermission
// and at least one of queryStoreAccessPermissions, on the database currently
// active on conn (set via a prior USE statement).
func hasRequiredPermissions(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	granted, err := queryGrantedPermissions(ctx, conn, queryTimeout, selectMyPermissionsQuery)
	if err != nil {
		return false, err
	}

	if !granted[requiredSchemaPermission] {
		return false, nil
	}

	for _, alternative := range queryStoreAccessPermissions {
		if granted[alternative] {
			return true, nil
		}
	}
	return false, nil
}

func queryGrantedPermissions(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration, query string) (map[string]bool, error) {
	granted := map[string]bool{}

	err := withQueryTimeout(ctx, queryTimeout, func(queryCtx context.Context) error {
		rows, err := conn.QueryContext(queryCtx, query)
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
		return nil, err
	}
	return granted, nil
}

// isQueryStoreEnabled checks whether Query Store is usable (READ_WRITE) on
// the database currently active on conn (set via a prior USE statement).
// It mirrors the "usable" definition used by the query_metrics preflight in
// checkQueryStoreState (query_store.go), rather than the raw ON/OFF state.
func isQueryStoreEnabled(ctx context.Context, conn *sql.Conn, queryTimeout time.Duration) (bool, error) {
	_, actualState, _, _, err := queryStoreState(ctx, conn, queryTimeout)
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
