package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_DATABASE_DETECTION = "database_detection"
	OP_SCHEMA_DETECTION   = "schema_detection"
	SchemaTableName       = "schema_table"
)

const (
	selectDatabaseName = `SELECT current_database()`

	selectSchemaNames = `
	SELECT 
	    schema_name 
	FROM 
	    information_schema.schemata
	WHERE 
	    schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
	    AND schema_name NOT LIKE 'pg_temp_%'
	    AND schema_name NOT LIKE 'pg_toast_temp_%'`
)

type SchemaTableArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type SchemaTable struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

type tableInfo struct {
	schema        string
	tableName     string
	tableType     string
	createTime    time.Time
	updateTime    time.Time
	b64CreateStmt string
	b64TableSpec  string
}

func NewSchemaTable(args SchemaTableArguments) (*SchemaTable, error) {
	c := &SchemaTable{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          log.With(args.Logger, "collector", SchemaTableName),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *SchemaTable) Name() string {
	return SchemaTableName
}

func (c *SchemaTable) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", SchemaTableName+" collector started")

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
			if err := c.extractNames(c.ctx); err != nil {
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

func (c *SchemaTable) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *SchemaTable) Stop() {
	c.cancel()
}

func (c *SchemaTable) extractNames(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectDatabaseName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query pg_database", "err", err)
		return err
	}
	defer rs.Close()

	var databases []string
	for rs.Next() {
		var dbName string
		if err := rs.Scan(&dbName); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_database", "err", err)
			break
		}
		databases = append(databases, dbName)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_DATABASE_DETECTION,
			c.instanceKey,
			fmt.Sprintf(`db="%s"`, dbName),
		)
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over result set", "err", err)
		return err
	}

	if len(databases) == 0 {
		level.Info(c.logger).Log("msg", "database is ok")
		return nil
	}

	schemaRs, err := c.dbConnection.QueryContext(ctx, selectSchemaNames)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schemas", "err", err)
		return err
	}
	defer schemaRs.Close()

	var schemas []string
	for schemaRs.Next() {
		var schema string
		if err := schemaRs.Scan(&schema); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schemata", "err", err)
			break
		}
		schemas = append(schemas, schema)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_SCHEMA_DETECTION,
			c.instanceKey,
			fmt.Sprintf(`schema="%s"`, schema),
		)
	}

	if err := schemaRs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over schemas result set", "err", err)
		return err
	}

	if len(schemas) == 0 {
		level.Info(c.logger).Log("msg", "no schema detected from information_schema.schemata")
		return nil
	}

	return nil
}
