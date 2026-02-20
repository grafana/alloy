package collector

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const StatStatementsRegistryName = "stat_statements_registry"

const statStatementsRefreshInterval = 1 * time.Minute

const selectStatStatements = `
	SELECT
		s.queryid,
		d.datname,
		s.calls,
		s.query,
		s.total_exec_time,
		s.mean_exec_time
	FROM pg_stat_statements s
		JOIN pg_database d ON s.dbid = d.oid AND NOT d.datistemplate AND d.datallowconn
	WHERE s.queryid IS NOT NULL
		AND d.datname NOT IN %s
`

type StatStatementsKey struct {
	QueryID int64
	DBName  string
}

type StatStatementsRow struct {
	QueryID     int64
	DBName      string
	Calls       int64
	Query       string
	TotalExecMs float64
	MeanExecMs  float64
}

type StatStatementsRegistryArguments struct {
	DB               *sql.DB
	ExcludeDatabases []string
	Logger           log.Logger
}

// StatStatementsRegistry periodically fetches pg_stat_statements and caches a
// per-(queryid, datname) snapshot with derived per-(queryid, datname) execution rates.
type StatStatementsRegistry struct {
	dbConnection     *sql.DB
	excludeDatabases []string
	logger           log.Logger
	running          *atomic.Bool
	ctx              context.Context
	cancel           context.CancelFunc

	mu        sync.RWMutex
	prevCalls map[StatStatementsKey]int64
	rates     map[StatStatementsKey]float64
	snapshot  map[StatStatementsKey]StatStatementsRow
	prevTime  time.Time
}

func NewStatStatementsRegistry(args StatStatementsRegistryArguments) (*StatStatementsRegistry, error) {
	return &StatStatementsRegistry{
		dbConnection:     args.DB,
		excludeDatabases: args.ExcludeDatabases,
		logger:           log.With(args.Logger, "collector", StatStatementsRegistryName),
		running:          &atomic.Bool{},
		prevCalls:        make(map[StatStatementsKey]int64),
		rates:            make(map[StatStatementsKey]float64),
		snapshot:         make(map[StatStatementsKey]StatStatementsRow),
	}, nil
}

func (r *StatStatementsRegistry) Name() string {
	return StatStatementsRegistryName
}

func (r *StatStatementsRegistry) Start(ctx context.Context) error {
	level.Debug(r.logger).Log("msg", "collector started")

	r.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	r.ctx = ctx
	r.cancel = cancel

	go func() {
		defer func() {
			r.Stop()
			r.running.Store(false)
		}()

		ticker := time.NewTicker(statStatementsRefreshInterval)
		defer ticker.Stop()

		for {
			if err := r.refresh(r.ctx); err != nil {
				level.Error(r.logger).Log("msg", "collector error", "err", err)
			}

			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return nil
}

func (r *StatStatementsRegistry) Stopped() bool {
	return !r.running.Load()
}

// Stop is idempotent.
func (r *StatStatementsRegistry) Stop() {
	r.cancel()
}

// GetExecutionRate returns the per-minute execution rate for the given
// (queryid, dbname) pair. Returns (0, false) until two snapshots are available.
func (r *StatStatementsRegistry) GetExecutionRate(queryid int64, dbname string) (float64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rate, ok := r.rates[StatStatementsKey{QueryID: queryid, DBName: dbname}]
	return rate, ok
}

// Snapshot returns a copy of the latest per-(queryid, dbid) pg_stat_statements data.
func (r *StatStatementsRegistry) Snapshot() map[StatStatementsKey]StatStatementsRow {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[StatStatementsKey]StatStatementsRow, len(r.snapshot))
	for k, v := range r.snapshot {
		out[k] = v
	}
	return out
}

func (r *StatStatementsRegistry) refresh(ctx context.Context) error {
	excludedDatabasesClause := buildExcludedDatabasesClause(r.excludeDatabases)
	query := fmt.Sprintf(selectStatStatements, excludedDatabasesClause)

	rows, err := r.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_statements: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	currentCalls := make(map[StatStatementsKey]int64)
	currentSnapshot := make(map[StatStatementsKey]StatStatementsRow)

	for rows.Next() {
		var row StatStatementsRow
		var queryText sql.NullString
		if err := rows.Scan(
			&row.QueryID, &row.DBName,
			&row.Calls, &queryText,
			&row.TotalExecMs, &row.MeanExecMs,
		); err != nil {
			level.Error(r.logger).Log("msg", "failed to scan pg_stat_statements row", "err", err)
			continue
		}
		if queryText.Valid {
			row.Query = queryText.String
		}
		key := StatStatementsKey{QueryID: row.QueryID, DBName: row.DBName}
		currentCalls[key] = row.Calls
		currentSnapshot[key] = row
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate pg_stat_statements rows: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.prevTime.IsZero() {
		elapsedMinutes := now.Sub(r.prevTime).Minutes()
		if elapsedMinutes > 0 {
			newRates := make(map[StatStatementsKey]float64, len(currentCalls))
			for key, calls := range currentCalls {
				prev, seen := r.prevCalls[key]
				if !seen {
					// First time seeing this (queryid, datname) — no delta yet.
					continue
				}
				delta := calls - prev
				if delta < 0 {
					// pg_stat_statements was reset; treat current calls as the delta.
					delta = calls
				}
				newRates[key] = float64(delta) / elapsedMinutes
			}
			r.rates = newRates
		}
	}

	r.prevCalls = currentCalls
	r.snapshot = currentSnapshot
	r.prevTime = now

	return nil
}
