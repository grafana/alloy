package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_SCHEMA_DETECTION = "schema_detection"
	OP_TABLE_DETECTION  = "table_detection"
	OP_CREATE_STATEMENT = "create_statement"
	SchemaTableName     = "schema_details"
)

const (
	selectDatabaseName = `SELECT current_database()`

	// selectSchemaNames gets all user-defined schemas, excluding system schemas
	selectSchemaNames = `
	SELECT
	    nspname as schema_name
	FROM
	    pg_catalog.pg_namespace
	WHERE
	    nspname NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
	    AND nspname NOT LIKE 'pg_temp_%'
	    AND nspname NOT LIKE 'pg_toast_%'`

	// selectTableNames gets table names for a specific schema
	/*
		AND pg_class.relkind IN ('r', 'v', 'm', 'f', 'p')  -- filter for application-facing objects
	*/
	selectTableNames = `
	SELECT
		pg_class.relname as table_name
	FROM pg_catalog.pg_class pg_class
	JOIN pg_catalog.pg_namespace pg_namespace ON pg_class.relnamespace = pg_namespace.oid
	WHERE pg_namespace.nspname = $1
		AND pg_class.relkind IN ('r', 'v', 'm', 'f', 'p')
		AND pg_class.relname NOT LIKE 'pg_%'`

	// selectColumnNames retrieves information about columns in a specified table
	/*
		pg_catalog.pg_get_expr: system function used to convert stored default value expressions back into human-readable SQL text
		attidentity: indicates if column is an IDENTITY column (part of auto-increment detection)
		pg_attribute: stores column information
		pg_attrdef: stores default values for columns
		pg_constraint: stores primary key information
		attr.attrelid = $1::regclass -- filter by the table name
		AND attr.attnum > 0  -- no system columns
		AND NOT attr.attisdropped -- no dropped columns`
	*/
	selectColumnNames = `
	SELECT
		attr.attname as column_name,
		attr.atttypid::regtype as column_type,
		attr.attnotnull as not_nullable,
		pg_catalog.pg_get_expr(def.adbin, def.adrelid) as column_default,
		attr.attidentity as identity_generation,
		CASE
		    WHEN constraint_pk.contype = 'p' THEN true
		    ELSE false
		END as is_primary_key
	FROM
		pg_attribute attr
		LEFT JOIN pg_catalog.pg_attrdef def ON attr.attrelid = def.adrelid AND attr.attnum = def.adnum
		LEFT JOIN pg_catalog.pg_constraint constraint_pk ON attr.attrelid = constraint_pk.conrelid AND attr.attnum = ANY(constraint_pk.conkey) AND constraint_pk.contype = 'p'
	WHERE
		attr.attrelid = $1::regclass
		AND attr.attnum > 0
		AND NOT attr.attisdropped`
)

type tableInfo struct {
	database     string
	schema       string
	tableName    string
	updateTime   time.Time
	b64TableSpec string
}

type tableSpec struct {
	Columns []columnSpec `json:"columns"`
}

type columnSpec struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	NotNull       bool   `json:"not_null,omitempty"`
	AutoIncrement bool   `json:"auto_increment,omitempty"`
	PrimaryKey    bool   `json:"primary_key,omitempty"`
	DefaultValue  string `json:"default_value,omitempty"`
}

type SchemaTableArguments struct {
	DB           *sql.DB
	EntryHandler loki.EntryHandler

	Logger log.Logger
}

type SchemaTable struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	entryHandler    loki.EntryHandler

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSchemaTable(args SchemaTableArguments) (*SchemaTable, error) {
	c := &SchemaTable{
		dbConnection:    args.DB,
		collectInterval: 10 * time.Minute, // TODO: make it configurable again once caching is implemented
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
	rs := c.dbConnection.QueryRowContext(ctx, selectDatabaseName)
	var dbName string
	if err := rs.Scan(&dbName); err != nil {
		return fmt.Errorf("failed to scan database name: %w", err)
	}

	schemaRs, err := c.dbConnection.QueryContext(ctx, selectSchemaNames)
	if err != nil {
		return fmt.Errorf("failed to query pg_namespace for database %s: %w", dbName, err)
	}
	defer schemaRs.Close()

	var schemas []string
	for schemaRs.Next() {
		var schema string
		if err := schemaRs.Scan(&schema); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_namespace", "database", dbName, "err", err)
			break
		}
		schemas = append(schemas, schema)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_SCHEMA_DETECTION,
			fmt.Sprintf(`database="%s" schema="%s"`, dbName, schema),
		)
	}

	if err := schemaRs.Err(); err != nil {
		return fmt.Errorf("error during iterating over pg_namespace result set for database %s: %w", dbName, err)
	}

	if len(schemas) == 0 {
		level.Info(c.logger).Log("msg", "no schema detected from pg_namespace", "database", dbName)
		return nil
	}

	tables := []*tableInfo{}

	for _, schema := range schemas {
		rs, err := c.dbConnection.QueryContext(ctx, selectTableNames, schema)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to query tables", "database", dbName, "schema", schema, "err", err)
			break
		}
		defer rs.Close()

		for rs.Next() {
			var tableName string
			if err := rs.Scan(&tableName); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan tables", "database", dbName, "schema", schema, "err", err)
				break
			}
			tables = append(tables, &tableInfo{
				database:   dbName,
				schema:     schema,
				tableName:  tableName,
				updateTime: time.Now(),
			})

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_TABLE_DETECTION,
				fmt.Sprintf(`database="%s" schema="%s" table="%s"`, dbName, schema, tableName),
			)
		}

		if err := rs.Err(); err != nil {
			return fmt.Errorf("failed to iterate over tables result set for database %s: %w", dbName, err)
		}
	}

	if len(tables) == 0 {
		level.Info(c.logger).Log("msg", "no tables detected from pg_tables", "database", dbName)
		return nil
	}

	for _, table := range tables {
		table, err = c.fetchTableDefinitions(ctx, table)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to get table definitions", "database", dbName, "schema", table.schema, "err", err)
			continue
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_CREATE_STATEMENT,
			fmt.Sprintf(
				`database="%s" schema="%s" table="%s" table_spec="%s"`,
				dbName, table.schema, table.tableName, table.b64TableSpec,
			),
		)
	}

	return nil
}

func (c *SchemaTable) fetchTableDefinitions(ctx context.Context, table *tableInfo) (*tableInfo, error) {
	spec, err := c.fetchColumnsDefinitions(ctx, table.database, table.schema, table.tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to analyze table spec", "database", table.database, "schema", table.schema, "table", table.tableName, "err", err)
		return table, err
	}

	jsonSpec, err := json.Marshal(spec)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to marshal table spec", "database", table.database, "schema", table.schema, "table", table.tableName, "err", err)
		return table, err
	}
	table.b64TableSpec = base64.StdEncoding.EncodeToString(jsonSpec)

	return table, nil
}

func (c *SchemaTable) fetchColumnsDefinitions(ctx context.Context, databaseName, schemaName, tableName string) (*tableSpec, error) {
	qualifiedTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	colRS, err := c.dbConnection.QueryContext(ctx, selectColumnNames, qualifiedTableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query table columns", "database", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer colRS.Close()

	tblSpec := &tableSpec{Columns: []columnSpec{}}

	for colRS.Next() {
		var columnName, columnType, identityGeneration string
		var columnDefault sql.NullString
		var notNullable, isPrimaryKey bool
		if err := colRS.Scan(&columnName, &columnType, &notNullable, &columnDefault, &identityGeneration, &isPrimaryKey); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan table columns", "database", databaseName, "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		defaultValue := ""
		if columnDefault.Valid {
			defaultValue = columnDefault.String
		}

		// detect auto-increment: either SERIAL or IDENTITY columns
		isAutoIncrement := (columnDefault.Valid && strings.Contains(strings.ToLower(columnDefault.String), "nextval(")) ||
			(identityGeneration == "a" || identityGeneration == "d")

		colSpec := columnSpec{
			Name:          columnName,
			Type:          columnType,
			NotNull:       notNullable,
			AutoIncrement: isAutoIncrement,
			PrimaryKey:    isPrimaryKey,
			DefaultValue:  defaultValue,
		}
		tblSpec.Columns = append(tblSpec.Columns, colSpec)
	}

	if err := colRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over table columns result set", "database", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	return tblSpec, nil
}
