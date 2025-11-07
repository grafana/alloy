package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/lib/pq"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	SchemaDetailsCollector = "schema_details"
	OP_SCHEMA_DETECTION    = "schema_detection"
	OP_TABLE_DETECTION     = "table_detection"
	OP_CREATE_STATEMENT    = "create_statement"
)

const (
	// selectAllDatabases makes use of the initial DB connection to discover other databases on the same Postgres instance
	selectAllDatabases = `
		SELECT datname 
		FROM pg_database 
		WHERE datistemplate = false
			AND has_database_privilege(datname, 'CONNECT')`

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

	// selectIndexes retrieves column-based and expression-based indexes on a specified table
	/*
		Postgres indexes can contain:
		1. Regular columns (pg_index.indkey[pos] != 0)
		2. Expressions (pg_index.indkey[pos] = 0)
		3. Mixed indexes with both columns and expressions
		pg_index.indkey: array of column numbers, 0 means expression
		generate_subscripts: creates positions 1,2,3... for each indkey element
		pg_get_indexdef(indexrelid, pos+1, true): gets expression text
		array_agg FILTERs: separate columns and expressions into different arrays
		column_nullables: array of nullability for each column (for Go processing)
		bool_or(NOT pg_attribute.attnotnull): true if ANY column is nullable
	*/
	selectIndexes = `
	SELECT
		index_relations.relname as index_name,
		pg_am.amname as index_type,
		pg_index.indisunique as unique,
		array_agg(
			CASE WHEN pg_index.indkey[pos] != 0
			THEN pg_attribute.attname
			END ORDER BY pos
		) FILTER (WHERE pg_index.indkey[pos] != 0) as column_names,
		array_agg(
			CASE WHEN pg_index.indkey[pos] = 0
			THEN pg_get_indexdef(pg_index.indexrelid, pos + 1, true)
			END ORDER BY pos
		) FILTER (WHERE pg_index.indkey[pos] = 0) as expressions,
		COALESCE(bool_or(NOT pg_attribute.attnotnull), false) as has_nullable_column
	FROM pg_class table_relations
	JOIN pg_index ON table_relations.oid = pg_index.indrelid
	JOIN pg_class index_relations ON index_relations.oid = pg_index.indexrelid
	JOIN pg_am ON index_relations.relam = pg_am.oid
	JOIN generate_subscripts(pg_index.indkey, 1) AS pos ON true
	LEFT JOIN pg_attribute ON table_relations.oid = pg_attribute.attrelid
		AND pg_attribute.attnum = pg_index.indkey[pos]
		AND NOT pg_attribute.attisdropped
	WHERE table_relations.relname = $2
		AND table_relations.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1)
	GROUP BY index_relations.relname, pg_am.amname, pg_index.indisunique
	ORDER BY index_name
`

	// selectForeignKeys retrieves foreign key constraints for a specified table
	/*
		pg_constraint stores all constraints
		join pg_class (table info) to get the source table
		join to pg_namespace (schema info) for schema filtering
		join to pg_class again to get referenced table
		use generate_subscripts() to correlate multi-column foreign keys by position
		pg_attribute joined twice to get column names for both source and referenced columns
	*/
	selectForeignKeys = `
	SELECT
		constraints.conname as constraint_name,
		source_column.attname as column_name,
		referenced_table.relname as referenced_table_name,
		referenced_column.attname as referenced_column_name
	FROM pg_constraint constraints
	JOIN pg_class source_table ON constraints.conrelid = source_table.oid
	JOIN pg_namespace schema ON source_table.relnamespace = schema.oid
	JOIN pg_class referenced_table ON constraints.confrelid = referenced_table.oid
	JOIN generate_subscripts(constraints.conkey, 1) AS position ON true
	JOIN pg_attribute source_column ON constraints.conrelid = source_column.attrelid
		AND source_column.attnum = constraints.conkey[position]
		AND NOT source_column.attisdropped
	JOIN pg_attribute referenced_column ON constraints.confrelid = referenced_column.attrelid
		AND referenced_column.attnum = constraints.confkey[position]
		AND NOT referenced_column.attisdropped
	WHERE constraints.contype = 'f'
		AND schema.nspname = $1
		AND source_table.relname = $2
	ORDER BY constraints.conname, position
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
	Columns     []columnSpec `json:"columns"`
	Indexes     []indexSpec  `json:"indexes,omitempty"`
	ForeignKeys []foreignKey `json:"foreign_keys,omitempty"`
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

type foreignKey struct {
	Name                 string `json:"name"`
	ColumnName           string `json:"column_name"`
	ReferencedTableName  string `json:"referenced_table_name"`
	ReferencedColumnName string `json:"referenced_column_name"`
}

// TableRegistry is a source-of-truth cache that keeps track of databases, schemas, tables
type TableRegistry struct {
	mu     sync.RWMutex
	tables map[string]map[string]map[string]bool // map[database]map[schema]map[table]bool
}

func NewTableRegistry() *TableRegistry {
	return &TableRegistry{
		tables: make(map[string]map[string]map[string]bool),
	}
}

func (tr *TableRegistry) SetTablesForDatabase(database string, tablesInfo []*tableInfo) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	delete(tr.tables, database)

	if len(tablesInfo) > 0 {
		tr.tables[database] = make(map[string]map[string]bool)
		for _, table := range tablesInfo {
			if tr.tables[database][table.schema] == nil {
				tr.tables[database][table.schema] = make(map[string]bool)
			}
			tr.tables[database][table.schema][table.tableName] = true
		}
	}
}

// IsValid returns whether or not a given database and parsed table name exists in the source-of-truth table registry
func (tr *TableRegistry) IsValid(database, parsedTableName string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	schemas, ok := tr.tables[database]
	if !ok {
		return false
	}

	schemaName, tableName := parseSchemaQualifiedIfAny(parsedTableName)
	switch schemaName {
	case "": // parsedTableName isn't schema-qualified, e.g. SELECT * FROM table_name.
		// table name can only be validated as "exists somewhere in the database", see limitation: https://github.com/grafana/grafana-dbo11y-app/issues/1838
		for _, tables := range schemas {
			if tables[tableName] {
				return true
			}
		}
	default: // parsedTableName is schema-qualified, e.g. SELECT * FROM schema_name.table_name
		if tables, ok := schemas[schemaName]; ok {
			return tables[tableName]
		}
	}

	return false
}

// parseSchemaQualifiedIfAny returns separated schema and table if the parsedTableName is schema-qualified, e.g. SELECT * FROM schema_name.table_name
func parseSchemaQualifiedIfAny(parsedTableName string) (string, string) {
	parts := strings.SplitN(parsedTableName, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parsedTableName
}

type SchemaDetailsArguments struct {
	DB              *sql.DB
	DSN             string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	CacheEnabled bool
	CacheSize    int
	CacheTTL     time.Duration

	Logger log.Logger

	dbConnectionFactory databaseConnectionFactory
}

type SchemaDetails struct {
	initialConnection   *sql.DB
	dbDSN               string
	dbConnectionFactory databaseConnectionFactory
	collectInterval     time.Duration
	entryHandler        loki.EntryHandler

	// Cache of table definitions. Entries are removed after a configurable TTL.
	// Key is a string of the form "database.schema.table".
	// (unlike MySQL) no create/update timestamp available for detecting immediately when a table schema is changed; relying on TTL only
	cache *expirable.LRU[string, *tableInfo]

	tableRegistry *TableRegistry

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSchemaDetails(args SchemaDetailsArguments) (*SchemaDetails, error) {
	factory := args.dbConnectionFactory
	if factory == nil {
		factory = defaultDbConnectionFactory
	}

	c := &SchemaDetails{
		initialConnection:   args.DB,
		dbDSN:               args.DSN,
		dbConnectionFactory: factory,
		collectInterval:     args.CollectInterval,
		entryHandler:        args.EntryHandler,
		tableRegistry:       NewTableRegistry(),
		logger:              log.With(args.Logger, "collector", SchemaDetailsCollector),
		running:             &atomic.Bool{},
	}

	if args.CacheEnabled {
		c.cache = expirable.NewLRU[string, *tableInfo](args.CacheSize, nil, args.CacheTTL)
	}

	return c, nil
}

func (c *SchemaDetails) Name() string {
	return SchemaDetailsCollector
}

func (c *SchemaDetails) GetTableRegistry() *TableRegistry {
	return c.tableRegistry
}

func (c *SchemaDetails) Start(ctx context.Context) error {
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

func (c *SchemaDetails) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *SchemaDetails) Stop() {
	c.cancel()
}

func (c *SchemaDetails) getAllDatabases(ctx context.Context) ([]string, error) {
	rows, err := c.initialConnection.QueryContext(ctx, selectAllDatabases)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to discover databases", "err", err)
		return nil, fmt.Errorf("failed to discover databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var datname string
		if err := rows.Scan(&datname); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan database name", "err", err)
			continue
		}
		databases = append(databases, datname)
	}

	if err := rows.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error iterating database rows", "err", err)
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}

func (c *SchemaDetails) extractSchemas(ctx context.Context, dbName string, dbConnection *sql.DB) error {
	schemaRs, err := dbConnection.QueryContext(ctx, selectSchemaNames)
	if err != nil {
		return fmt.Errorf("failed to query pg_namespace for database %s: %w", dbName, err)
	}
	defer schemaRs.Close()

	var schemas []string
	for schemaRs.Next() {
		var schema string
		if err := schemaRs.Scan(&schema); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan pg_namespace", "datname", dbName, "err", err)
			break
		}
		schemas = append(schemas, schema)

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_SCHEMA_DETECTION,
			fmt.Sprintf(`datname="%s" schema="%s"`, dbName, schema),
		)
	}

	if err := schemaRs.Err(); err != nil {
		return fmt.Errorf("error during iterating over pg_namespace result set for database %s: %w", dbName, err)
	}

	if len(schemas) == 0 {
		level.Info(c.logger).Log("msg", "no schema detected from pg_namespace", "datname", dbName)
		return nil
	}

	tables := []*tableInfo{}

	for _, schema := range schemas {
		rs, err := dbConnection.QueryContext(ctx, selectTableNames, schema)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to query tables", "datname", dbName, "schema", schema, "err", err)
			break
		}
		defer rs.Close()

		for rs.Next() {
			var tableName string
			if err := rs.Scan(&tableName); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan tables", "datname", dbName, "schema", schema, "err", err)
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
				fmt.Sprintf(`datname="%s" schema="%s" table="%s"`, dbName, schema, tableName),
			)
		}

		if err := rs.Err(); err != nil {
			return fmt.Errorf("failed to iterate over tables result set for database %s: %w", dbName, err)
		}
	}

	c.tableRegistry.SetTablesForDatabase(dbName, tables)

	if len(tables) == 0 {
		level.Info(c.logger).Log("msg", "no tables detected from pg_tables", "datname", dbName)
		return nil
	}

	for _, table := range tables {
		cacheKey := fmt.Sprintf("%s.%s.%s", table.database, table.schema, table.tableName)

		cacheHit := false
		if c.cache != nil {
			if cached, ok := c.cache.Get(cacheKey); ok {
				table = cached
				cacheHit = true
			}
		}

		if !cacheHit {
			table, err = c.fetchTableDefinitions(ctx, table, dbConnection)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to get table definitions", "datname", dbName, "schema", table.schema, "err", err)
				continue
			}
			if c.cache != nil {
				c.cache.Add(cacheKey, table)
			}
		}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_CREATE_STATEMENT,
			fmt.Sprintf(
				`datname="%s" schema="%s" table="%s" table_spec="%s"`,
				dbName, table.schema, table.tableName, table.b64TableSpec,
			),
		)
	}

	return nil
}

func (c *SchemaDetails) extractNames(ctx context.Context) error {
	databases, err := c.getAllDatabases(ctx)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to discover databases", "err", err)
		return fmt.Errorf("failed to discover databases: %w", err)
	}

	for _, dbName := range databases {
		databaseDSN, err := replaceDatabaseNameInDSN(c.dbDSN, dbName)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to create DSN for database", "datname", dbName, "err", err)
			continue
		}

		conn, err := c.dbConnectionFactory(databaseDSN)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to create connection to database", "datname", dbName, "err", err)
			continue
		}

		if err := c.extractSchemas(ctx, dbName, conn); err != nil {
			level.Error(c.logger).Log("msg", "failed to collect schema from database", "datname", dbName, "err", err)
			if conn != c.initialConnection {
				conn.Close()
			}
			continue
		}

		if conn != c.initialConnection {
			if err := conn.Close(); err != nil {
				level.Warn(c.logger).Log("msg", "failed to close database connection", "datname", dbName, "err", err)
			}
		}
	}

	return nil
}

func (c *SchemaDetails) fetchTableDefinitions(ctx context.Context, table *tableInfo, dbConnection *sql.DB) (*tableInfo, error) {
	spec, err := c.fetchColumnsDefinitions(ctx, table.database, table.schema, table.tableName, dbConnection)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to analyze table spec", "datname", table.database, "schema", table.schema, "table", table.tableName, "err", err)
		return table, err
	}

	jsonSpec, err := json.Marshal(spec)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to marshal table spec", "datname", table.database, "schema", table.schema, "table", table.tableName, "err", err)
		return table, err
	}
	table.b64TableSpec = base64.StdEncoding.EncodeToString(jsonSpec)

	return table, nil
}

func (c *SchemaDetails) fetchColumnsDefinitions(ctx context.Context, databaseName, schemaName, tableName string, dbConnection *sql.DB) (*tableSpec, error) {
	qualifiedTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
	colRS, err := dbConnection.QueryContext(ctx, selectColumnNames, qualifiedTableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query table columns", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer colRS.Close()

	tblSpec := &tableSpec{Columns: []columnSpec{}}

	for colRS.Next() {
		var columnName, columnType, identityGeneration string
		var columnDefault sql.NullString
		var notNullable, isPrimaryKey bool
		if err := colRS.Scan(&columnName, &columnType, &notNullable, &columnDefault, &identityGeneration, &isPrimaryKey); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan table columns", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
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
		level.Error(c.logger).Log("msg", "failed to iterate over table columns result set", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	indexesRS, err := dbConnection.QueryContext(ctx, selectIndexes, schemaName, tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query indexes", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer indexesRS.Close()

	for indexesRS.Next() {
		var indexName, indexType string
		var unique, hasNullableColumn bool
		var columns, expressions pq.StringArray

		if err := indexesRS.Scan(&indexName, &indexType, &unique, &columns, &expressions, &hasNullableColumn); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan indexes", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		// nullable if has nullable columns or has expressions
		nullable := hasNullableColumn || len(expressions) > 0 // assume that indexes with any expressions are nullable, TODO: investigate nullability of expressions

		tblSpec.Indexes = append(tblSpec.Indexes, indexSpec{
			Name:        indexName,
			Type:        indexType,
			Unique:      unique,
			Columns:     columns,
			Expressions: expressions,
			Nullable:    nullable,
		})
	}

	if err := indexesRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over indexes", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	fkRS, err := dbConnection.QueryContext(ctx, selectForeignKeys, schemaName, tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query foreign keys", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer fkRS.Close()

	for fkRS.Next() {
		var constraintName, columnName, referencedTableName, referencedColumnName string
		if err := fkRS.Scan(&constraintName, &columnName, &referencedTableName, &referencedColumnName); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan foreign keys", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		tblSpec.ForeignKeys = append(tblSpec.ForeignKeys, foreignKey{
			Name:                 constraintName,
			ColumnName:           columnName,
			ReferencedTableName:  referencedTableName,
			ReferencedColumnName: referencedColumnName,
		})
	}

	if err := fkRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over foreign keys result set", "datname", databaseName, "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	return tblSpec, nil
}
