package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/go-sqllexer"
	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	ORDER BY total_exec_time DESC
	LIMIT 100
`

type QueryDetailsArguments struct {
	DB               *sql.DB
	CollectInterval  time.Duration
	ExcludeDatabases []string
	EntryHandler     loki.EntryHandler
	TableRegistry    *TableRegistry

	Logger log.Logger
}

type QueryDetails struct {
	dbConnection     *sql.DB
	collectInterval  time.Duration
	excludeDatabases []string
	entryHandler     loki.EntryHandler
	tableRegistry    *TableRegistry
	normalizer       *sqllexer.Normalizer

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQueryDetails(args QueryDetailsArguments) (*QueryDetails, error) {
	return &QueryDetails{
		dbConnection:     args.DB,
		collectInterval:  args.CollectInterval,
		excludeDatabases: args.ExcludeDatabases,
		entryHandler:     args.EntryHandler,
		tableRegistry:    args.TableRegistry,
		normalizer:       sqllexer.NewNormalizer(sqllexer.WithCollectTables(true), sqllexer.WithCollectComments(true)),
		logger:           log.With(args.Logger, "collector", QueryDetailsCollector),
		running:          &atomic.Bool{},
	}, nil
}

func (c *QueryDetails) Name() string {
	return QueryDetailsCollector
}

func (c *QueryDetails) Start(ctx context.Context) error {
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
			if err := c.fetchAndAssociate(c.ctx); err != nil {
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

func (c *QueryDetails) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QueryDetails) Stop() {
	c.cancel()
}

func (c QueryDetails) fetchAndAssociate(ctx context.Context) error {
	query := fmt.Sprintf(selectQueriesFromActivity, buildExcludedDatabasesClause(c.excludeDatabases))
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
			level.Error(c.logger).Log("msg", "failed to scan result set for pg_stat_statements", "err", err)
			continue
		}

		queryText, err = removeComments(c.normalizer, queryText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to remove comments", "err", err)
			continue
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_ASSOCIATION,
			fmt.Sprintf(`queryid="%s" querytext=%q datname="%s"`, queryID, queryText, databaseName),
		)

		tables, err := tokenizeTableNames(c.normalizer, queryText)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to tokenize table names", "err", err)
			continue
		}

		for _, table := range tables {
			validated := false
			if c.tableRegistry != nil {
				validated = c.tableRegistry.IsValid(databaseName, table)
			}

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				fmt.Sprintf(`queryid="%s" datname="%s" table="%s" validated="%t"`, queryID, databaseName, table, validated),
			)
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over result set", "err", err)
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
