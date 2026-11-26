package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/go-sqllexer"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	QueryDetailsCollector = "query_details"
)

var selectQueriesFromActivity = `
	SELECT
		pg_stat_statements.queryid,
		pg_stat_statements.query,
		pg_database.datname
	FROM
		pg_stat_statements
	JOIN pg_database
		ON pg_database.oid = pg_stat_statements.dbid
	WHERE
		total_exec_time > (
			SELECT percentile_cont(0.1)
				WITHIN GROUP (ORDER BY total_exec_time)
				FROM pg_stat_statements
		)
		AND pg_database.datname NOT IN %s
		%s
	ORDER BY total_exec_time DESC
	LIMIT %d
`

type QueryDetailsArguments struct {
	DB                        *sql.DB
	CollectInterval           time.Duration
	StatementsLimit           int
	ExcludeDatabases          []string
	IncludeDatabases          []string
	ExcludeUsers              []string
	IncludeUsers              []string
	EntryHandler              loki.EntryHandler
	TableRegistry             *TableRegistry
	EnableErrorLogsProcessing bool

	Logger *slog.Logger
}

type queryDetailsKey struct {
	database database
	queryID  string
}

type QueryDetails struct {
	dbConnection              *sql.DB
	collectInterval           time.Duration
	statementsLimit           int
	excludeDatabases          []string
	includeDatabases          []string
	excludeUsers              []string
	includeUsers              []string
	entryHandler              loki.EntryHandler
	tableRegistry             *TableRegistry
	enableErrorLogsProcessing bool
	normalizer                *sqllexer.Normalizer
	// lastEmittedAt records the wall-clock time at which OP_QUERY_ASSOCIATION
	// was last emitted for a (database, queryid), used to throttle logging
	// to at most one emission per EmitInterval per query.
	lastEmittedAt map[queryDetailsKey]time.Time
	// unvalidated tracks queries whose parsed tables were emitted before schema
	// collection populated the database in tableRegistry, so they can be
	// re-emitted once when schema collection becomes available.
	unvalidated map[queryDetailsKey]struct{}
	now         func() time.Time

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQueryDetails(args QueryDetailsArguments) (*QueryDetails, error) {
	return &QueryDetails{
		dbConnection:              args.DB,
		collectInterval:           args.CollectInterval,
		statementsLimit:           args.StatementsLimit,
		excludeDatabases:          args.ExcludeDatabases,
		includeDatabases:          args.IncludeDatabases,
		excludeUsers:              args.ExcludeUsers,
		includeUsers:              args.IncludeUsers,
		entryHandler:              args.EntryHandler,
		tableRegistry:             args.TableRegistry,
		enableErrorLogsProcessing: args.EnableErrorLogsProcessing,
		normalizer:                sqllexer.NewNormalizer(sqllexer.WithCollectTables(true), sqllexer.WithCollectComments(true), sqllexer.WithKeepIdentifierQuotation(true)),
		lastEmittedAt:             map[queryDetailsKey]time.Time{},
		unvalidated:               map[queryDetailsKey]struct{}{},
		now:                       time.Now,
		logger:                    args.Logger.With("collector", QueryDetailsCollector),
		running:                   &atomic.Bool{},
	}, nil
}

func (c *QueryDetails) Name() string {
	return QueryDetailsCollector
}

func (c *QueryDetails) Start(ctx context.Context) error {
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
			if err := c.fetchAndAssociate(c.ctx); err != nil {
				c.logger.Error("collector error", "err", err)
			}

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

func (c *QueryDetails) Stopped() bool {
	return !c.running.Load()
}

func (c *QueryDetails) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *QueryDetails) fetchAndAssociate(ctx context.Context) error {
	databasesClause := buildDatabaseFilterClause(c.excludeDatabases, c.includeDatabases, "pg_database.datname")
	usersClause := buildUserFilterClause(c.excludeUsers, c.includeUsers, "pg_get_userbyid(pg_stat_statements.userid)")
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	query := fmt.Sprintf(selectQueriesFromActivity, databasesClause, usersClause, c.statementsLimit) // #nosec G201 -- filter values are quote-escaped before interpolation
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch statements from pg_stat_statements view: %w", err)
	}
	defer rs.Close()

	now := c.now()
	for rs.Next() {
		var queryID, queryText string
		var databaseName database
		err := rs.Scan(&queryID, &queryText, &databaseName)
		if err != nil {
			c.logger.Error("failed to scan result set for pg_stat_statements", "err", err)
			continue
		}

		key := queryDetailsKey{database: databaseName, queryID: queryID}
		hasDatabase := c.tableRegistry != nil && c.tableRegistry.SchemasForDatabase(databaseName) != nil
		if last, ok := c.lastEmittedAt[key]; ok && now.Sub(last) < database_observability.EmitInterval {
			if _, pending := c.unvalidated[key]; !pending ||
				!hasDatabase {
				continue
			}
		}

		pendingValidation := c.emit(databaseName, queryID, queryText, hasDatabase)
		c.lastEmittedAt[key] = now
		if pendingValidation {
			c.unvalidated[key] = struct{}{}
		} else {
			delete(c.unvalidated, key)
		}
	}

	if err := rs.Err(); err != nil {
		c.logger.Error("failed to iterate over result set", "err", err)
		return err
	}

	c.pruneThrottle(now)
	return nil
}

// emit logs the query association and any table names parsed from the query.
// It returns true when parsed tables were emitted without schema data, so the
// query can be re-emitted once schema collection populates tableRegistry.
func (c *QueryDetails) emit(databaseName database, queryID, queryText string, hasDatabase bool) bool {
	var fp string
	if c.enableErrorLogsProcessing {
		// Fingerprint the raw text BEFORE comment stripping; pg_query
		// canonicalizes literals at the AST level so the value is stable
		// across comment-only differences and matches the fingerprint
		// computed elsewhere from pg_stat_activity / server logs.
		var fpErr error
		fp, fpErr = fingerprint.Fingerprint(queryText)
		if fpErr != nil {
			c.logger.Warn("could not compute query fingerprint; emitting query_association without fingerprint", "queryid", queryID, "err", fpErr)
		} else if fp == "" {
			c.logger.Warn("empty query fingerprint; emitting query_association without fingerprint", "queryid", queryID)
		}
	}

	queryText, err := removeComments(c.normalizer, queryText)
	if err != nil {
		c.logger.Error("failed to remove comments", "err", err)
		return false
	}

	var body string
	if fp != "" {
		body = fmt.Sprintf(`queryid=%q query_fingerprint=%q querytext=%q datname=%q`, queryID, fp, queryText, databaseName)
	} else {
		body = fmt.Sprintf(`queryid=%q querytext=%q datname=%q`, queryID, queryText, databaseName)
	}
	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		database_observability.OP_QUERY_ASSOCIATION,
		body,
	)

	tables, err := tokenizeTableNames(c.normalizer, queryText)
	if err != nil {
		c.logger.Error("failed to tokenize table names", "err", err)
		return false
	}

	if len(tables) == 0 {
		return false
	}

	parsedTables := make([]database_observability.ParsedTable, 0, len(tables))
	for _, table := range tables {
		validated := false
		resolvedTable := table
		if hasDatabase {
			resolvedTable, validated = c.tableRegistry.IsValid(databaseName, table)
		}
		parsedTables = append(parsedTables, database_observability.ParsedTable{Table: resolvedTable, Validated: validated})
	}
	tablesJSON, err := json.Marshal(parsedTables)
	if err != nil {
		c.logger.Error("failed to encode parsed tables", "err", err)
		return false
	}
	c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
		logging.LevelInfo,
		database_observability.OP_QUERY_PARSED_TABLE_NAMES_V2,
		fmt.Sprintf(`queryid=%q datname=%q tables=%q`, queryID, databaseName, tablesJSON),
	)

	return c.tableRegistry != nil && !hasDatabase
}

// pruneThrottle drops entries whose last emission was at least EmitInterval ago.
func (c *QueryDetails) pruneThrottle(now time.Time) {
	for key, last := range c.lastEmittedAt {
		if now.Sub(last) >= database_observability.EmitInterval {
			delete(c.lastEmittedAt, key)
			delete(c.unvalidated, key)
		}
	}
}

func tokenizeTableNames(normalizer *sqllexer.Normalizer, sqlText string) ([]string, error) {
	sqlText = strings.TrimSuffix(sqlText, "...")
	_, metadata, err := normalizer.Normalize(sqlText, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize table names: %w", err)
	}

	return metadata.Tables, nil
}

func removeComments(normalizer *sqllexer.Normalizer, sqlText string) (string, error) {
	_, metadata, err := normalizer.Normalize(sqlText, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return sqlText, fmt.Errorf("failed to normalize sql text: %w", err)
	}

	if len(metadata.Comments) == 0 {
		return sqlText, nil
	}

	for _, comment := range metadata.Comments {
		sqlText = strings.ReplaceAll(sqlText, comment, "")
	}

	return strings.TrimSpace(sqlText), nil
}
