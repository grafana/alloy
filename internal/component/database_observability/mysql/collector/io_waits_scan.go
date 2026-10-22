package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// selectIOWaits reads the I/O wait summary of every table and index of every
// schema that is not excluded. The rows where INDEX_NAME is NULL are the
// table-level rows ("no index used"); the others are one row per index.
//
// OBJECT_TYPE = 'TABLE' leaves out temporary tables and also lets the server
// use the table's index, which starts with OBJECT_TYPE; see selectIOWaitsSchema.
const selectIOWaits = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE OBJECT_TYPE = 'TABLE' AND OBJECT_SCHEMA NOT IN %s`

// selectIOWaitsSchema reads the same rows for one schema. Both conditions are
// needed: the table's only index is (OBJECT_TYPE, OBJECT_SCHEMA, OBJECT_NAME,
// INDEX_NAME), so without OBJECT_TYPE the server scans the whole table and
// builds a row for every table of every schema, which is slow on servers with
// many tables, and NOT IN can't use the index at all.
const selectIOWaitsSchema = `
	SELECT OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME, COUNT_FETCH
	FROM performance_schema.table_io_waits_summary_by_index_usage
	WHERE OBJECT_TYPE = 'TABLE' AND OBJECT_SCHEMA = ?`

// selectSchemaNames lists the schemas that have tables, cheaply: the table is
// one row per table, in an ordinary InnoDB table.
const selectSchemaNames = `
	SELECT DISTINCT database_name
	FROM mysql.innodb_table_stats`

// perSchemaTimeout bounds the read of one schema, so that a single slow schema
// does not use up the time of the whole run.
const perSchemaTimeout = 30 * time.Second

type ioWaitsRow struct {
	schema     string
	object     string
	index      sql.NullString
	countFetch uint64
}

// IOWaitsScan reads performance_schema.table_io_waits_summary_by_index_usage
// and shares the result between the table_stats and index_stats collectors.
//
// The scan is the most expensive query of both collectors on servers with many
// tables, and both need the same rows, so it runs once per collection cycle:
// a caller that asks while a scan is in flight waits for it, and a caller that
// asks soon after a scan finished gets its result.
//
// Each collector registers a SchemaFilter. When every registered filter leaves
// something out, the scan reads only the schemas that at least one of them
// selects, one schema at a time. Otherwise it reads everything in one
// statement, which is the cheaper choice when little is left out.
type IOWaitsScan struct {
	db             *sql.DB
	excludeSchemas []string
	logger         *slog.Logger

	filtersMu sync.Mutex
	filters   []SchemaFilter

	// sem is a one-slot lock that, unlike a sync.Mutex, can be abandoned when
	// the caller's context is done while another caller's scan is running.
	sem  chan struct{}
	rows []ioWaitsRow
	at   time.Time

	// bySchema holds the rows of the last scan per schema, so that a schema
	// whose read fails keeps its previous rows. Only used by per-schema scans.
	bySchema map[string][]ioWaitsRow
}

func NewIOWaitsScan(db *sql.DB, excludeSchemas []string, logger *slog.Logger) *IOWaitsScan {
	return &IOWaitsScan{
		db:             db,
		excludeSchemas: excludeSchemas,
		logger:         logger.With("scan", "table_io_waits_summary_by_index_usage"),
		sem:            make(chan struct{}, 1),
	}
}

// use registers the schemas a collector wants.
func (s *IOWaitsScan) use(filter SchemaFilter) {
	s.filtersMu.Lock()
	defer s.filtersMu.Unlock()
	s.filters = append(s.filters, filter)
}

// selection returns whether the scan has to read a schema, which is when any
// registered filter selects it, and whether every filter leaves something out.
func (s *IOWaitsScan) selection() (selects func(schema string) bool, restricted bool) {
	s.filtersMu.Lock()
	filters := slices.Clone(s.filters)
	s.filtersMu.Unlock()

	restricted = len(filters) > 0
	for _, f := range filters {
		if !f.Restricted() {
			restricted = false
		}
	}

	return func(schema string) bool {
		return slices.ContainsFunc(filters, func(f SchemaFilter) bool { return f.Match(schema) })
	}, restricted
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

	selects, restricted := s.selection()

	var rows []ioWaitsRow
	var err error
	if restricted {
		rows, err = s.scanSchemas(ctx, selects)
	} else {
		s.bySchema = nil
		rows, err = s.scanAll(ctx)
	}
	if err != nil {
		return nil, err
	}

	s.rows, s.at = rows, time.Now()
	return rows, nil
}

// scanAll reads every schema that is not excluded in one statement.
func (s *IOWaitsScan) scanAll(ctx context.Context) ([]ioWaitsRow, error) {
	return s.query(ctx, fmt.Sprintf(selectIOWaits, buildExcludedSchemasClause(s.excludeSchemas)))
}

// scanSchemas reads the selected schemas one at a time. A schema that fails
// keeps the rows of its last successful read; the run only fails when it ran
// out of time.
func (s *IOWaitsScan) scanSchemas(ctx context.Context, selects func(string) bool) ([]ioWaitsRow, error) {
	names, err := s.schemaNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	var selected []string
	for _, schema := range names {
		if selects(schema) {
			selected = append(selected, schema)
		}
	}
	if len(selected) == 0 {
		s.bySchema = nil
		return nil, nil
	}

	// Prepared once and run for every schema: a statement with an argument
	// would otherwise be prepared, run and closed again for each schema, three
	// round trips instead of one.
	stmt, err := s.db.PrepareContext(ctx, selectIOWaitsSchema)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	var result []ioWaitsRow
	bySchema := make(map[string][]ioWaitsRow, len(selected))
	for _, schema := range selected {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		rows, err := s.scanSchema(ctx, stmt, schema)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.logger.Error("failed to read schema, keeping its previous rows", "schema", schema, "err", err)
			rows = s.bySchema[schema]
		}
		bySchema[schema] = rows
		result = append(result, rows...)
	}

	s.bySchema = bySchema
	return result, nil
}

func (s *IOWaitsScan) scanSchema(ctx context.Context, stmt *sql.Stmt, schema string) ([]ioWaitsRow, error) {
	ctx, cancel := context.WithTimeout(ctx, perSchemaTimeout)
	defer cancel()

	rows, err := stmt.QueryContext(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return readIOWaitsRows(rows)
}

// schemaNames lists the schemas that have tables, leaving out the ones that are
// always excluded.
func (s *IOWaitsScan) schemaNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, selectSchemaNames)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if slices.Contains(excludedSchemas, name) || slices.Contains(s.excludeSchemas, name) {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate: %w", err)
	}
	return names, nil
}

func (s *IOWaitsScan) query(ctx context.Context, query string) ([]ioWaitsRow, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return readIOWaitsRows(rows)
}

// readIOWaitsRows reads all rows and closes them.
func readIOWaitsRows(rows *sql.Rows) ([]ioWaitsRow, error) {
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
