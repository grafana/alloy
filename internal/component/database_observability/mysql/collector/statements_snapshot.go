package collector

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// selectTopStatements selects the queries that spent the most time running
// among the ones seen in the last 24 hours, the way the perf_schema.eventsstatements
// collector of the embedded exporter does with its default settings, so that
// both work on the same queries.
const selectTopStatements = `
	SELECT
		SCHEMA_NAME,
		DIGEST,
		QUERY_SAMPLE_TEXT,
		LAST_SEEN
	FROM performance_schema.events_statements_summary_by_digest
	WHERE LAST_SEEN > DATE_SUB(NOW(), INTERVAL 1 DAY)
	AND SCHEMA_NAME NOT IN %s
	ORDER BY SUM_TIMER_WAIT DESC
	LIMIT %d`

// Statement is a query of the digest table.
type Statement struct {
	SchemaName string
	// Digest and QuerySampleText are empty when MySQL has none for the query.
	Digest          string
	QuerySampleText string
	LastSeen        time.Time
}

type StatementsSnapshotArguments struct {
	DB             *sql.DB
	Limit          int
	ExcludeSchemas []string
	// MaxAge is how long a read is reused before the table is read again.
	MaxAge time.Duration
}

// StatementsSnapshot reads the top queries of the digest table for the
// collectors that work on them, which share one read instead of each querying
// the table.
type StatementsSnapshot struct {
	db             *sql.DB
	limit          int
	excludeSchemas []string
	maxAge         time.Duration
	now            func() time.Time

	mu         sync.Mutex
	statements []Statement
	readAt     time.Time
}

func NewStatementsSnapshot(args StatementsSnapshotArguments) *StatementsSnapshot {
	return &StatementsSnapshot{
		db:             args.DB,
		limit:          args.Limit,
		excludeSchemas: args.ExcludeSchemas,
		maxAge:         args.MaxAge,
		now:            time.Now,
	}
}

// Get returns the top queries, by total running time, reading the table again
// when the last read is older than MaxAge. The returned slice must not be
// modified.
func (s *StatementsSnapshot) Get(ctx context.Context) ([]Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if !s.readAt.IsZero() && now.Sub(s.readAt) < s.maxAge {
		return s.statements, nil
	}

	statements, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	s.statements, s.readAt = statements, now
	return statements, nil
}

func (s *StatementsSnapshot) read(ctx context.Context) ([]Statement, error) {
	query := fmt.Sprintf(selectTopStatements, buildExcludedSchemasClause(s.excludeSchemas), s.limit)
	rs, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top statements: %w", err)
	}
	defer rs.Close()

	statements := make([]Statement, 0, s.limit)
	for rs.Next() {
		var st Statement
		var digest, querySampleText sql.NullString
		if err := rs.Scan(&st.SchemaName, &digest, &querySampleText, &st.LastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan top statements: %w", err)
		}
		st.Digest, st.QuerySampleText = digest.String, querySampleText.String
		statements = append(statements, st)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate top statements: %w", err)
	}
	return statements, nil
}
