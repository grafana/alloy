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
	"github.com/lib/pq"
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
	SchemaTableName     = "schema_table"
)

const (
	selectDatabaseName = `SELECT current_database()`

	selectSchemaNames = `
	SELECT 
	    nspname as schema_name
	FROM 
	    pg_catalog.pg_namespace
	WHERE 
	    nspname NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
	    AND nspname NOT LIKE 'pg_temp_%'
	    AND nspname NOT LIKE 'pg_toast_%'`

	selectTableNames = `
	SELECT 
		pg_class.relname as table_name
	FROM pg_catalog.pg_class pg_class
	JOIN pg_catalog.pg_namespace pg_namespace ON pg_class.relnamespace = pg_namespace.oid
	WHERE pg_namespace.nspname = $1 
		AND pg_class.relkind IN ('r', 'v', 'm', 'f', 'p')  -- filter for application-facing objects
		AND pg_class.relname NOT LIKE 'pg_%'`

	selectColumnNames = `
	SELECT
		attr.attname as column_name,
		attr.atttypid::regtype as column_type,
		attr.attnotnull as not_nullable,
		pg_catalog.pg_get_expr(def.adbin, def.adrelid) as column_default, -- PostgreSQL system function used to convert stored default value expressions back into human-readable SQL text
		attr.attidentity as identity_generation, -- IDENTITY column will be flagged as auto-increment: identity generation type, if any
		CASE 
		    WHEN constraint_pk.contype = 'p' THEN true 
		    ELSE false 
		END as is_primary_key
	FROM
		pg_attribute attr -- pg_attribute stores column information
		LEFT JOIN pg_catalog.pg_attrdef def ON attr.attrelid = def.adrelid AND attr.attnum = def.adnum -- pg_attrdef stores default values for columns
		LEFT JOIN pg_catalog.pg_constraint constraint_pk ON attr.attrelid = constraint_pk.conrelid AND attr.attnum = ANY(constraint_pk.conkey) AND constraint_pk.contype = 'p' -- pg_constraint stores primary key information
	WHERE
		attr.attrelid = $1::regclass -- filter by the table name
		AND attr.attnum > 0  -- no system columns
		AND NOT attr.attisdropped -- no dropped columns`

	selectIndexesBasicInfo = `
	SELECT 
		index_relations.relname as index_name,
		pg_am.amname as index_type,
		pg_index.indisunique as unique
	FROM pg_class table_relations -- pg_class entry for tables
	JOIN pg_index ON table_relations.oid = pg_index.indrelid -- pg_index has additional information about indexes
	JOIN pg_class index_relations ON index_relations.oid = pg_index.indexrelid -- pg_class entry for indexes
	JOIN pg_am ON index_relations.relam = pg_am.oid -- stores information about index access methods
	WHERE table_relations.relname = $2 -- filter by table name
		AND table_relations.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1) -- filter by schema name
	ORDER BY index_name -- consistent output
`

	selectIndexColumns = `
	SELECT 
		index_relations.relname as index_name,
		pg_am.amname as index_type,
		pg_index.indisunique as unique,
		array_agg(pg_attribute.attname ORDER BY array_position(pg_index.indkey, pg_attribute.attnum)) as column_names
	FROM pg_class table_relations -- pg_class entry for tables
	JOIN pg_index ON table_relations.oid = pg_index.indrelid -- pg_index has additional information about indexes
	JOIN pg_class index_relations ON index_relations.oid = pg_index.indexrelid -- pg_class entry for indexes
	JOIN pg_am ON index_relations.relam = pg_am.oid -- stores information about index access methods
	JOIN pg_attribute ON table_relations.oid = pg_attribute.attrelid -- pg_attribute stores column information
		AND pg_attribute.attnum = ANY(pg_index.indkey) -- match column number with pg_index columns array
		AND pg_attribute.attnum != 0 -- only regular columns, not expressions
	WHERE table_relations.relname = $2 -- filter by table name
		AND table_relations.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1) -- filter by schema name
		AND NOT pg_attribute.attisdropped -- no dropped columns
	GROUP BY index_relations.relname, pg_am.amname, pg_index.indisunique
	ORDER BY index_name -- consistent output
`

	// Get expression-based index components
	selectIndexExpressions = `
	SELECT 
		index_relations.relname as index_name,
		pos as seq_in_index,
		pg_get_indexdef(pg_index.indexrelid, pos, true) as expression
	FROM pg_class table_relations -- pg_class entry for tables
	JOIN pg_index ON table_relations.oid = pg_index.indrelid -- pg_index has additional information about indexes
	JOIN pg_class index_relations ON index_relations.oid = pg_index.indexrelid -- pg_class entry for indexes
	JOIN generate_subscripts(pg_index.indkey, 1) AS pos ON true -- generate position for each index component
	WHERE table_relations.relname = $2 -- filter by table name
		AND table_relations.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1) -- filter by schema name
		AND pg_index.indkey[pos-1] = 0 -- only expression-based components (0 = expression, not column)
	ORDER BY index_relations.relname, pos -- consistent output ordered by index name and component position
`
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
	Indexes []indexSpec  `json:"indexes,omitempty"`
}

type columnSpec struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	NotNull       bool   `json:"not_null,omitempty"`
	AutoIncrement bool   `json:"auto_increment,omitempty"`
	PrimaryKey    bool   `json:"primary_key,omitempty"`
	DefaultValue  string `json:"default_value,omitempty"`
}

type indexSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Columns     []string `json:"columns"`
	Expressions []string `json:"expressions,omitempty"`
	Unique      bool     `json:"unique"`
	Nullable    bool     `json:"nullable"`
}

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
	rs := c.dbConnection.QueryRowContext(ctx, selectDatabaseName)
	var dbName string
	if err := rs.Scan(&dbName); err != nil {
		level.Error(c.logger).Log("msg", "failed to scan database name", "err", err)
		return err
	}

	schemaRs, err := c.dbConnection.QueryContext(ctx, selectSchemaNames)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query pg_namespace", "database", dbName, "err", err)
		return err
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
			c.instanceKey,
			fmt.Sprintf(`database="%s" schema="%s"`, dbName, schema),
		)
	}

	if err := schemaRs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over pg_namespace result set", "database", dbName, "err", err)
		return err
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
				c.instanceKey,
				fmt.Sprintf(`database="%s" schema="%s" table="%s"`, dbName, schema, tableName),
			)
		}

		if err := rs.Err(); err != nil {
			level.Error(c.logger).Log("msg", "error during iterating over tables result set", "err", err)
			return err
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
			c.instanceKey,
			fmt.Sprintf(
				`database="%s" schema="%s" table="%s" table_spec="%s"`, // TODO: No create statement here -- if we don't need table_spec, we may be able to remove this
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
		level.Error(c.logger).Log("msg", "error during iterating over table columns result set", "database", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	indexes := make(map[string]*indexSpec)

	// Get column-based indexes with array aggregation
	columnsRS, err := c.dbConnection.QueryContext(ctx, selectIndexColumns, schemaName, tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query index columns", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer columnsRS.Close()

	for columnsRS.Next() {
		var indexName, indexType string
		var unique bool
		var columnNames pq.StringArray

		if err := columnsRS.Scan(&indexName, &indexType, &unique, &columnNames); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan index columns", "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		indexes[indexName] = &indexSpec{
			Name:    indexName,
			Type:    indexType,
			Unique:  unique,
			Columns: columnNames,
			// Nullable: TODO: how do we handle nullable and multi-column indexes? also for MySQL
		}
	}
	if err := columnsRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over index columns", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	// Get expression-based components
	expressionsRS, err := c.dbConnection.QueryContext(ctx, selectIndexExpressions, schemaName, tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query index expressions", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer expressionsRS.Close()

	for expressionsRS.Next() {
		var indexName, expression string
		var seqInIndex int

		if err := expressionsRS.Scan(&indexName, &seqInIndex, &expression); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan index expressions", "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		if idx, exists := indexes[indexName]; exists {
			// Ensure we have enough space in the slice
			for len(idx.Expressions) < seqInIndex {
				idx.Expressions = append(idx.Expressions, "")
			}
			// Set the expression at the correct position (seq_in_index is 1-based)
			if seqInIndex > 0 && seqInIndex <= len(idx.Expressions) {
				idx.Expressions[seqInIndex-1] = expression
			}
			// Expressions are generally nullable
			idx.Nullable = true
		}
	}

	if err := expressionsRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over index expressions", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	// Convert map to slice for consistent ordering
	for _, idx := range indexes {
		tblSpec.Indexes = append(tblSpec.Indexes, *idx)
	}

	return tblSpec, nil
}
