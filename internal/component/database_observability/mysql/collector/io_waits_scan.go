package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// selectIOWaits reads the I/O wait summary of every table and index. The rows
// where INDEX_NAME is NULL are the table-level rows ("no index used"); the
// others are one row per index.
const selectIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE OBJECT_SCHEMA NOT IN %s`

type ioWaitsRow struct {
	schema     string
	object     string
	index      sql.NullString
	countFetch uint64
}

// IOWaitsScan scans performance_schema.table_io_waits_summary_by_index_usage
// and shares the result between the table_stats and index_stats collectors.
//
// The scan is the most expensive query of both collectors on servers with many
// tables, and both need the same rows, so it runs once per collection cycle:
// a caller that asks while a scan is in flight waits for it, and a caller that
// asks soon after a scan finished gets its result.
type IOWaitsScan struct {
	db             *sql.DB
	excludeSchemas []string

	// sem is a one-slot lock that, unlike a sync.Mutex, can be abandoned when
	// the caller's context is done while another caller's scan is running.
	sem  chan struct{}
	rows []ioWaitsRow
	at   time.Time
}

func NewIOWaitsScan(db *sql.DB, excludeSchemas []string) *IOWaitsScan {
	return &IOWaitsScan{
		db:             db,
		excludeSchemas: excludeSchemas,
		sem:            make(chan struct{}, 1),
	}
}

// get returns the rows of a scan that finished less than maxAge ago, or runs a
// new scan. The returned slice must not be modified.
func (s *IOWaitsScan) get(ctx context.Context, maxAge time.Duration) ([]ioWaitsRow, error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	if !s.at.IsZero() && time.Since(s.at) < maxAge {
		return s.rows, nil
	}

	rows, err := s.scan(ctx)
	if err != nil {
		return nil, err
	}
	s.rows, s.at = rows, time.Now()
	return rows, nil
}

func (s *IOWaitsScan) scan(ctx context.Context) ([]ioWaitsRow, error) {
	query := fmt.Sprintf(selectIOWaits, buildExcludedSchemasClause(s.excludeSchemas))
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var result []ioWaitsRow
	for rows.Next() {
		var r ioWaitsRow
		if err := rows.Scan(&r.schema, &r.object, &r.index, &r.countFetch); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	return result, nil
}
