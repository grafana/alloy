package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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

type HealthCheckArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type HealthCheck struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	entryHandler    loki.EntryHandler
	logger          log.Logger

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewHealthCheck(args HealthCheckArguments) (*HealthCheck, error) {
	h := &HealthCheck{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          log.With(args.Logger, "collector", HealthCheckCollector),
		running:         &atomic.Bool{},
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

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
		}()

		ticker := time.NewTicker(c.collectInterval)

		for {
			c.fetchHealthChecks(c.ctx)
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()

	return nil
}

func (c *HealthCheck) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *HealthCheck) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
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
		checkMonitoringUserPrivileges,
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
	const q = `SELECT * FROM pg_stat_statements LIMIT 1`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		r.err = fmt.Errorf("query pg_stat_statements: %w", err)
		return r
	}
	defer rows.Close()

	if rows.Next() {
		r.result = true
	}

	if err := rows.Err(); err != nil {
		r.err = fmt.Errorf("iterate pg_stat_statements: %w", err)
	}

	r.result = true

	return r
}
