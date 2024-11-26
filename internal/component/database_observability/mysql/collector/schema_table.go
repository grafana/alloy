package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_SCHEMA_DETECTION = "schema_detection"
	OP_TABLE_DETECTION  = "table_detection"
	OP_CREATE_STATEMENT = "create_statement"
)

const (
	selectSchemaName = `
	SELECT
		SCHEMA_NAME
	FROM
		information_schema.schemata
	WHERE
		SCHEMA_NAME NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`

	selectTableName = `
	SELECT
		TABLE_NAME,
		CREATE_TIME,
		ifnull(UPDATE_TIME, CREATE_TIME) AS UPDATE_TIME
	FROM
		information_schema.tables
	WHERE
		TABLE_SCHEMA = ?`

	// Note that the fully qualified table name is appendend to the query,
	// for some reason it doesn't work with placeholders.
	showCreateTable = `SHOW CREATE TABLE`
)

type SchemaTableArguments struct {
	DSN            string
	ScrapeInterval time.Duration
	EntryHandler   loki.EntryHandler
	CacheTTL       time.Duration

	Logger log.Logger
}

type SchemaTable struct {
	dbConnection   *sql.DB
	scrapeInterval time.Duration
	entryHandler   loki.EntryHandler
	// Cache of table definitions. Entries are removed after a configurable TTL.
	// Key is a string of the form "schema.table@timestamp", where timestamp is
	// the last update time of the table (this allows capturing schema changes
	// at each scan, regardless of caching).
	// TODO(cristian): allow configuring cache size (currently unlimited).
	cache *expirable.LRU[string, tableInfo]

	logger log.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

type tableInfo struct {
	schema     string
	tableName  string
	createTime time.Time
	updateTime time.Time
	createStmt string
}

func NewSchemaTable(args SchemaTableArguments) (*SchemaTable, error) {
	dbConnection, err := sql.Open("mysql", args.DSN+"?parseTime=true")
	if err != nil {
		return nil, err
	}
	if dbConnection == nil {
		return nil, errors.New("nil DB connection")
	}

	if err = dbConnection.Ping(); err != nil {
		return nil, err
	}

	return &SchemaTable{
		dbConnection:   dbConnection,
		scrapeInterval: args.ScrapeInterval,
		entryHandler:   args.EntryHandler,
		cache:          expirable.NewLRU[string, tableInfo](0, nil, args.CacheTTL),
		logger:         args.Logger,
	}, nil
}

func (c *SchemaTable) Run(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "SchemaTable collector running")

	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		ticker := time.NewTicker(c.scrapeInterval)

		for {
			if err := c.extractSchema(c.ctx); err != nil {
				break
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

func (c *SchemaTable) Stop() {
	c.cancel()
	c.dbConnection.Close()
}

func (c *SchemaTable) extractSchema(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectSchemaName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schemata", "err", err)
		return err
	}
	defer rs.Close()

	var schemas []string
	for rs.Next() {
		if err := rs.Err(); err != nil {
			level.Error(c.logger).Log("msg", "failed to iterate rs", "err", err)
			break
		}

		var schema string
		if err := rs.Scan(&schema); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schemata", "err", err)
			break
		}
		schemas = append(schemas, schema)

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": "integrations/db-o11y"},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line:      fmt.Sprintf(`level=info msg="schema detected" op="%s" schema="%s"`, OP_SCHEMA_DETECTION, schema),
			},
		}
	}

	tables := []tableInfo{}

	for _, schema := range schemas {
		rs, err := c.dbConnection.QueryContext(ctx, selectTableName, schema)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to query tables", "err", err)
			break
		}
		defer rs.Close()

		for rs.Next() {
			if err := rs.Err(); err != nil {
				level.Error(c.logger).Log("msg", "failed to iterate rs", "err", err)
				break
			}

			var table string
			var createTime, updateTime time.Time
			if err := rs.Scan(&table, &createTime, &updateTime); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan tables", "err", err)
				break
			}
			tables = append(tables, tableInfo{schema: schema, tableName: table, createTime: createTime, updateTime: updateTime})

			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"job": "integrations/db-o11y"},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line:      fmt.Sprintf(`level=info msg="table detected" op="%s" schema="%s" table="%s"`, OP_TABLE_DETECTION, schema, table),
				},
			}
		}
	}

	for _, table := range tables {
		fullyQualifiedTable := fmt.Sprintf("%s.%s", table.schema, table.tableName)
		cacheKey := fmt.Sprintf("%s@%d", fullyQualifiedTable, table.updateTime.Unix())

		if c.cache.Contains(cacheKey) {
			level.Info(c.logger).Log("msg", "table definition already in cache", "schema", table.schema, "table", table.tableName)
			continue
		}

		row := c.dbConnection.QueryRowContext(ctx, showCreateTable+" "+fullyQualifiedTable)
		if row.Err() != nil {
			level.Error(c.logger).Log("msg", "failed to show create table", "err", row.Err())
			break
		}

		var tableName string
		var createStmt string
		if err = row.Scan(&tableName, &createStmt); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan create table", "err", err)
			break
		}

		table.createStmt = createStmt
		c.cache.Add(cacheKey, table)

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": "integrations/db-o11y"},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line:      fmt.Sprintf(`level=info msg="create table" op="%s" schema="%s" table="%s" create_statement="%s"`, OP_CREATE_STATEMENT, table.schema, table.tableName, createStmt),
			},
		}
	}

	return nil
}
