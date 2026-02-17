package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
		checkRequiredGrants,
		checkEventsStatementsDigestHasRows,
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

// checkRequiredGrants verifies required privileges are present.
// Requires: PROCESS, REPLICATION CLIENT, SHOW VIEW on *.*
// and SELECT on performance_schema.*
func checkRequiredGrants(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "RequiredGrantsPresent"}
	req := map[string]bool{
		"PROCESS":            false,
		"REPLICATION CLIENT": false,
		"SELECT":             false,
		"SHOW VIEW":          false,
	}

	rows, err := db.QueryContext(ctx, "SHOW GRANTS")
	if err != nil {
		r.err = fmt.Errorf("SHOW GRANTS: %w", err)
		return r
	}
	defer rows.Close()

	for rows.Next() {
		var grantLine string
		if err := rows.Scan(&grantLine); err != nil {
			r.err = fmt.Errorf("scan SHOW GRANTS: %w", err)
			return r
		}
		up := strings.ToUpper(grantLine)

		if strings.Contains(up, "SELECT") {
			if strings.Contains(up, " ON `PERFORMANCE_SCHEMA`.*") ||
				strings.Contains(up, " ON PERFORMANCE_SCHEMA.*") ||
				strings.Contains(up, " ON `PERFORMANCE_SCHEMA`.") ||
				strings.Contains(up, " ON *.*") {

				req["SELECT"] = true
			}
		}

		if strings.Contains(up, "ALL PRIVILEGES") {
			if strings.Contains(up, " ON `PERFORMANCE_SCHEMA`.*") ||
				strings.Contains(up, " ON PERFORMANCE_SCHEMA.*") ||
				strings.Contains(up, " ON `PERFORMANCE_SCHEMA`.") ||
				strings.Contains(up, " ON *.*") {

				req["SELECT"] = true
			}
		}

		if strings.Contains(up, "SHOW VIEW") {
			req["SHOW VIEW"] = true
		}

		if strings.Contains(up, "PROCESS") && strings.Contains(up, " ON *.*") {
			req["PROCESS"] = true
		}

		if strings.Contains(up, "REPLICATION CLIENT") && strings.Contains(up, " ON *.*") {
			req["REPLICATION CLIENT"] = true
		}
	}
	if err := rows.Err(); err != nil {
		r.err = fmt.Errorf("iterate SHOW GRANTS: %w", err)
		return r
	}

	r.result = req["PROCESS"] && req["REPLICATION CLIENT"] && req["SELECT"] && req["SHOW VIEW"]

	if !r.result {
		var missing []string
		if !req["PROCESS"] {
			missing = append(missing, "PROCESS")
		}
		if !req["REPLICATION CLIENT"] {
			missing = append(missing, "REPLICATION CLIENT")
		}
		if !req["SELECT"] {
			missing = append(missing, "SELECT on performance_schema.*")
		}
		if !req["SHOW VIEW"] {
			missing = append(missing, "SHOW VIEW")
		}
		r.value = fmt.Sprintf("missing grants: %s", strings.Join(missing, ", "))
	}

	return r
}

// checkEventsStatementsDigestHasRows ensures performance_schema.events_statements_summary_by_digest has rows.
func checkEventsStatementsDigestHasRows(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "PerformanceSchemaHasRows"}
	const q = `SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest`
	var rowCount int64
	if err := db.QueryRowContext(ctx, q).Scan(&rowCount); err != nil {
		r.err = err
		return r
	}
	if rowCount == 0 {
		return r
	}
	r.result = true
	return r
}
