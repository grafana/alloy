package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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
	OP_QUERY_ASSOCIATION       = "query_association"
	OP_QUERY_PARSED_TABLE_NAME = "query_parsed_table_name"
	QueryTablesName            = "query_tables"
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
	ORDER BY total_exec_time DESC
	LIMIT 100
`

type QueryTablesArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type QueryTables struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler
	normalizer      *sqllexer.Normalizer

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewQueryTables(args QueryTablesArguments) (*QueryTables, error) {
	return &QueryTables{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		normalizer:      sqllexer.NewNormalizer(sqllexer.WithCollectTables(true)),
		logger:          log.With(args.Logger, "collector", QueryTablesName),
		running:         &atomic.Bool{},
	}, nil
}

func (c *QueryTables) Name() string {
	return QueryTablesName
}

func (c *QueryTables) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", QueryTablesName+" collector started")

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

func (c *QueryTables) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *QueryTables) Stop() {
	c.cancel()
}

func (c QueryTables) fetchAndAssociate(ctx context.Context) error {
	slog.Info("Fetching and associating queries")
	rs, err := c.dbConnection.QueryContext(ctx, selectQueriesFromActivity)
	if err != nil {
		slog.Error("failed to fetch statements from pg_stat_statements view", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		var queryID, queryText, databaseName string
		err := rs.Scan(
			&queryID,
			&queryText,
			&databaseName,
		)
		if err != nil {
			slog.Error("failed to scan result set for pg_stat_statements", "err", err)
			continue
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_QUERY_ASSOCIATION,
			c.instanceKey,
			fmt.Sprintf(`queryid="%s" querytext="%s" datname="%s" engine="postgres"`, queryID, queryText, databaseName),
		)

		tables, err := c.tryTokenizeTableNames(queryText)
		if err != nil {
			slog.Error("failed to tokenize table names", "err", err)
			continue
		}

		for _, table := range tables {
			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_QUERY_PARSED_TABLE_NAME,
				c.instanceKey,
				fmt.Sprintf(`queryid="%s" datname="%s" table="%s" engine="postgres"`, queryID, databaseName, table),
			)
		}
	}

	if err := rs.Err(); err != nil {
		slog.Error("failed to iterate rs", "err", err)
		return err
	}

	return nil
}

func (c QueryTables) tryTokenizeTableNames(sqlText string) ([]string, error) {
	sqlText = strings.TrimSuffix(sqlText, "...")
	_, metadata, err := c.normalizer.Normalize(sqlText, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize sql text: %w", err)
	}

	return metadata.Tables, nil
}
