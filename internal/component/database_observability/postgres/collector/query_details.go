package collector

import (
	"context"
	"database/sql"
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
	QueryDetailsCollector      = "query_details"
	OP_QUERY_ASSOCIATION       = "query_association"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
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
	ExcludeUsers              []string
	EntryHandler              loki.EntryHandler
	TableRegistry             *TableRegistry
	EnableErrorLogsProcessing bool

	Logger *slog.Logger
}

type QueryDetails struct {
	dbConnection              *sql.DB
	collectInterval           time.Duration
	statementsLimit           int
	excludeDatabases          []string
	excludeUsers              []string
	entryHandler              loki.EntryHandler
	tableRegistry             *TableRegistry
	enableErrorLogsProcessing bool
	normalizer                *sqllexer.Normalizer

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
		excludeUsers:              args.ExcludeUsers,
		entryHandler:              args.EntryHandler,
		tableRegistry:             args.TableRegistry,
		enableErrorLogsProcessing: args.EnableErrorLogsProcessing,
		normalizer:                sqllexer.NewNormalizer(sqllexer.WithCollectTables(true), sqllexer.WithCollectComments(true), sqllexer.WithKeepIdentifierQuotation(true)),
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
	excludedDatabasesClause := buildExcludedDatabasesClause(c.excludeDatabases)
	excludedUsersClause := buildExcludedUsersClause(c.excludeUsers, "pg_get_userbyid(pg_stat_statements.userid)")
	query := fmt.Sprintf(selectQueriesFromActivity, excludedDatabasesClause, excludedUsersClause, c.statementsLimit)
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch statements from pg_stat_statements view: %w", err)
	}
	defer rs.Close()

	for rs.Next() {
		var queryID, queryText string
		var databaseName database
		err := rs.Scan(&queryID, &queryText, &databaseName)
		if err != nil {
			c.logger.Error("failed to scan result set for pg_stat_statements", "err", err)
			continue
		}

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

		queryText, err = removeComments(c.normalizer, queryText)
		if err != nil {
			c.logger.Error("failed to remove comments", "err", err)
			continue
		}

		var body string
		if fp != "" {
			body = fmt.Sprintf(`queryid="%s" query_fingerprint="%s" querytext=%q datname="%s"`, queryID, fp, queryText, databaseName)
		} else {
			body = fmt.Sprintf(`queryid="%s" querytext=%q datname="%s"`, queryID, queryText, databaseName)
		}
		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_ASSOCIATION,
			body,
		)

		tables, err := tokenizeTableNames(c.normalizer, queryText)
		if err != nil {
			c.logger.Error("failed to tokenize table names", "err", err)
			continue
		}

		for _, table := range tables {
			validated := false
			resolvedTable := table
			if c.tableRegistry != nil {
				resolvedTable, validated = c.tableRegistry.IsValid(databaseName, table)
			}

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				fmt.Sprintf(`queryid="%s" datname="%s" table="%s" validated="%t"`, queryID, databaseName, resolvedTable, validated),
			)
		}
	}

	if err := rs.Err(); err != nil {
		c.logger.Error("failed to iterate over result set", "err", err)
		return err
	}

	return nil
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
