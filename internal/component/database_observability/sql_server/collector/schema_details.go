package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

const (
	SchemaDetailsCollector = "schema_details"
	OP_TABLE_DETECTION     = "table_detection"
)

// selectTablesTemplate lists every table and view visible in the connected
// database, excluding the schemas in the IN-clause appended at format time.
// We read from INFORMATION_SCHEMA.TABLES for portability across all supported
// SQL Server versions and Azure SQL.
const selectTablesTemplate = `
SELECT
	TABLE_CATALOG,
	TABLE_SCHEMA,
	TABLE_NAME,
	TABLE_TYPE
FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_SCHEMA NOT IN %s
ORDER BY TABLE_SCHEMA, TABLE_NAME`

type SchemaDetailsArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	ExcludeSchemas  []string
	EntryHandler    loki.EntryHandler

	Logger *slog.Logger
}

type SchemaDetails struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	excludeSchemas  []string
	entryHandler    loki.EntryHandler

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewSchemaDetails(args SchemaDetailsArguments) (*SchemaDetails, error) {
	c := &SchemaDetails{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		excludeSchemas:  args.ExcludeSchemas,
		entryHandler:    args.EntryHandler,
		logger:          args.Logger.With("collector", SchemaDetailsCollector),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *SchemaDetails) Name() string {
	return SchemaDetailsCollector
}

func (c *SchemaDetails) Start(ctx context.Context) error {
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
			if err := c.extractSchema(c.ctx); err != nil {
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

func (c *SchemaDetails) Stopped() bool {
	return !c.running.Load()
}

func (c *SchemaDetails) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *SchemaDetails) extractSchema(ctx context.Context) error {
	query := fmt.Sprintf(selectTablesTemplate, buildExcludedSchemasClause(c.excludeSchemas))
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rs.Close()

	tableCount := 0
	for rs.Next() {
		var database, schema, tableName, tableType string
		if err := rs.Scan(&database, &schema, &tableName, &tableType); err != nil {
			c.logger.Error("failed to scan tables", "err", err)
			break
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_TABLE_DETECTION,
			fmt.Sprintf(`database="%s" schema="%s" table="%s"`, database, schema, tableName),
		)
		tableCount++
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over tables result set: %w", err)
	}

	if tableCount == 0 {
		c.logger.Info("no tables detected from information_schema.tables")
	}

	return nil
}
