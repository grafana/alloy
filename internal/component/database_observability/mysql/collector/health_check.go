package collector

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Extracts leading major.minor.patch from a version string (e.g., "8.0.36" from "8.0.36-28.1").
var mysqlVersionRegex = regexp.MustCompile(`^((\d+)(\.\d+)(\.\d+))`)

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
			if err := c.fetchHealthChecks(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "collector error", "err", err)
			}

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

func (c *HealthCheck) fetchHealthChecks(ctx context.Context) error {
	checks := []func(context.Context, *sql.DB) healthCheckResult{
		checkDBConnectionValid,
		checkPerformanceSchemaEnabled,
		checkMySQLVersion,
		checkAlloyVersion,
		checkRequiredGrants,
		checkDigestVariablesLength,
		checkSetupConsumerCPUTimeEnabled,
		checkSetupConsumersEventsWaitsEnabled,
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

	return nil
}

// checkPerformanceSchemaEnabled validates that performance_schema is enabled.
func checkPerformanceSchemaEnabled(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "PerformaneSchemaEnabled"}
	const q = `SHOW VARIABLES LIKE 'performance_schema'`
	var varName, varValue string
	if err := db.QueryRowContext(ctx, q).Scan(&varName, &varValue); err != nil {
		r.err = fmt.Errorf("query performance_schema variable: %w", err)
		return r
	}

	r.result = strings.EqualFold(varValue, "ON") || varValue == "1"
	r.value = varValue
	return r
}

// checkDBConnectionValid validates the database connection with a short timeout.
func checkDBConnectionValid(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "DBConnectionValid"}
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(subCtx); err != nil {
		r.value = err.Error()
		return r
	}
	r.result = true
	r.value = "ok"
	return r
}

// checkMySQLVersion validates that the database the MySQL version >= 8.0.
func checkMySQLVersion(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "MySQLVersionSupported"}
	const q = `SELECT VERSION()`
	var version string
	if err := db.QueryRowContext(ctx, q).Scan(&version); err != nil {
		r.err = fmt.Errorf("query version(): %w", err)
		return r
	}

	matches := mysqlVersionRegex.FindStringSubmatch(version)
	if len(matches) <= 1 {
		r.err = fmt.Errorf("unexpected version format: %s", version)
		return r
	}

	parsed, err := semver.ParseTolerant(matches[1])
	if err != nil {
		r.err = fmt.Errorf("parse semver: %w", err)
		return r
	}

	r.result = semver.MustParseRange(">=8.0.0")(parsed)
	r.value = version
	return r
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

		// Mark individual privileges if present on *.* scope.
		for k := range req {
			if strings.Contains(up, " ON *.*") && strings.Contains(up, k) {
				req[k] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		r.err = fmt.Errorf("iterate SHOW GRANTS: %w", err)
		return r
	}

	r.result = true
	for k, found := range req {
		if !found {
			r.result = false
			if r.value == "" {
				r.value = "missing: " + k
			} else {
				r.value += "," + k
			}
		}
	}

	return r
}

// checkDigestVariablesLength ensures text/digest length variables are >= 4096.
func checkDigestVariablesLength(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "DigestVariablesLengthCheck"}
	const q = `
SELECT
	@@performance_schema_max_sql_text_length,
	@@performance_schema_max_digest_length,
	@@max_digest_length`

	var sqlTextLen, digestLen, maxDigestLen sql.NullInt64
	if err := db.QueryRowContext(ctx, q).Scan(&sqlTextLen, &digestLen, &maxDigestLen); err != nil {
		r.err = fmt.Errorf("query perf schema length vars: %w", err)
		return r
	}

	r.result = true
	if sqlTextLen.Int64 < 4096 {
		r.result = false
		r.value += fmt.Sprintf("performance_schema_max_sql_text_length=%d < 4096", sqlTextLen.Int64)
	}
	if digestLen.Int64 < 4096 {
		r.result = false
		r.value += fmt.Sprintf(" performance_schema_max_digest_length=%d < 4096", digestLen.Int64)
	}
	if maxDigestLen.Int64 < 4096 {
		r.result = false
		r.value += fmt.Sprintf(" max_digest_length=%d < 4096", maxDigestLen.Int64)
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

// checkSetupConsumerCPUTimeEnabled validates events_statements_cpu consumer is enabled.
func checkSetupConsumerCPUTimeEnabled(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "SetupConsumerCPUTimeEnabled"}
	const q = `SELECT enabled FROM performance_schema.setup_consumers WHERE NAME = 'events_statements_cpu'`
	var enabled string
	if err := db.QueryRowContext(ctx, q).Scan(&enabled); err != nil {
		r.err = fmt.Errorf("query setup_consumers: %w", err)
		return r
	}
	r.result = enabled == "YES"
	r.value = enabled
	return r
}

// checkSetupConsumersEventsWaitsEnabled validates events_waits_current and events_waits_history consumers are enabled.
func checkSetupConsumersEventsWaitsEnabled(ctx context.Context, db *sql.DB) healthCheckResult {
	r := healthCheckResult{name: "SetupConsumersEventsWaitsEnabled"}
	const q = `SELECT name, enabled FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current','events_waits_history')`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		r.err = fmt.Errorf("query setup_consumers: %w", err)
		return r
	}
	defer rows.Close()

	r.result = true

	for rows.Next() {
		var consumerName, enabled string
		if err := rows.Scan(&consumerName, &enabled); err != nil {
			r.err = fmt.Errorf("scan setup_consumers: %w", err)
			return r
		}
		if enabled != "YES" {
			r.result = false
			r.value += fmt.Sprintf(" %v=%v", consumerName, enabled)
		}
	}
	if err := rows.Err(); err != nil {
		r.err = fmt.Errorf("iterate setup_consumers: %w", err)
		return r
	}

	return r
}
