package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_DATABASE_DETECTION = "database_detection"
	OP_SCHEMA_DETECTION   = "schema_detection"
	OP_TABLE_DETECTION    = "table_detection"
	OP_CREATE_STATEMENT   = "create_statement"
	TodoName              = "schema_table"
)

const (
	selectDatabaseNames = `
	SELECT 
	    datname 
	FROM 
	    pg_database
	WHERE datname NOT IN ('template0', 'template1', 'postgres', 'rdsadmin', 'azure_maintenance', 'azure_sys') -- TODO: should we exclude the cloud provider dbs? and what about postgres?`

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

type todoCollectorArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler

	CacheEnabled bool
	CacheSize    int
	CacheTTL     time.Duration

	Logger log.Logger
}

type todoName struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler

	// Cache of table definitions. Entries are removed after a configurable TTL.
	// Key is a string of the form "schema.table@timestamp", where timestamp is
	// the last update time of the table (this allows capturing schema changes
	// at each scan, regardless of caching).
	cache *expirable.LRU[string, *tableInfo]

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

func NewTodoName(args todoCollectorArguments) (*todoName, error) {
	c := &todoName{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		logger:          log.With(args.Logger, "collector", TodoName),
		running:         &atomic.Bool{},
	}

	if args.CacheEnabled {
		c.cache = expirable.NewLRU[string, *tableInfo](args.CacheSize, nil, args.CacheTTL)
	}

	return c, nil
}

func (c *todoName) Name() string {
	return TodoName
}

func (c *todoName) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", TodoName+" collector started")

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

func (c *todoName) Stopped() bool {
	return !c.running.Load()
}

// Stop should be kept idempotent
func (c *todoName) Stop() {
	c.cancel()
}

func (c *todoName) extractNames(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectDatabaseNames) // TODO: alternately, just select current_database()?
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

	// TODO: There will only be the one database, the one that we are connected to.
	// That means that we might be able to skip the database detection step above
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

	return nil
}

//func (c *todoName) fetchTableDefinitions(ctx context.Context, fullyQualifiedTable string, table *tableInfo) (*tableInfo, error) {
//	row := c.dbConnection.QueryRowContext(ctx, showCreateTable+" "+fullyQualifiedTable)
//	if err := row.Err(); err != nil {
//		level.Error(c.logger).Log("msg", "failed to show create table", "schema", table.schema, "table", table.tableName, "err", err)
//		return table, err
//	}
//
//	var tableName, createStmt, characterSetClient, collationConnection string
//	switch table.tableType {
//	case "BASE TABLE":
//		if err := row.Scan(&tableName, &createStmt); err != nil {
//			level.Error(c.logger).Log("msg", "failed to scan create table", "schema", table.schema, "table", table.tableName, "err", err)
//			return table, err
//		}
//	case "VIEW":
//		if err := row.Scan(&tableName, &createStmt, &characterSetClient, &collationConnection); err != nil {
//			level.Error(c.logger).Log("msg", "failed to scan create view", "schema", table.schema, "table", table.tableName, "err", err)
//			return table, err
//		}
//	default:
//		level.Error(c.logger).Log("msg", "unknown table type", "schema", table.schema, "table", table.tableName, "table_type", table.tableType)
//		return nil, fmt.Errorf("unknown table type: %s", table.tableType)
//	}
//	table.b64CreateStmt = base64.StdEncoding.EncodeToString([]byte(createStmt))
//
//	spec, err := c.fetchColumnsDefinitions(ctx, table.schema, table.tableName)
//	if err != nil {
//		level.Error(c.logger).Log("msg", "failed to analyze table spec", "schema", table.schema, "table", table.tableName, "err", err)
//		return table, err
//	}
//	jsonSpec, err := json.Marshal(spec)
//	if err != nil {
//		level.Error(c.logger).Log("msg", "failed to marshal table spec", "schema", table.schema, "table", table.tableName, "err", err)
//		return table, err
//	}
//	table.b64TableSpec = base64.StdEncoding.EncodeToString(jsonSpec)
//
//	return table, nil
//}
//
//func (c *todoName) fetchColumnsDefinitions(ctx context.Context, schemaName string, tableName string) (*tableSpec, error) {
//	colRS, err := c.dbConnection.QueryContext(ctx, selectColumnNames, schemaName, tableName)
//	if err != nil {
//		level.Error(c.logger).Log("msg", "failed to query table columns", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//	defer colRS.Close()
//
//	tblSpec := &tableSpec{Columns: []columnSpec{}}
//
//	for colRS.Next() {
//		var columnName, isNullable, columnType, columnKey, extra string
//		var columnDefault sql.NullString
//		if err := colRS.Scan(&columnName, &columnDefault, &isNullable, &columnType, &columnKey, &extra); err != nil {
//			level.Error(c.logger).Log("msg", "failed to scan table columns", "schema", schemaName, "table", tableName, "err", err)
//			return nil, err
//		}
//
//		extra = strings.ToUpper(extra) // "extra" might contain a variety of textual information
//		defaultValue := ""
//		if columnDefault.Valid {
//			defaultValue = columnDefault.String
//			if strings.Contains(extra, "ON UPDATE CURRENT_TIMESTAMP") {
//				defaultValue += " ON UPDATE CURRENT_TIMESTAMP"
//			}
//		}
//
//		colSpec := columnSpec{
//			Name:          columnName,
//			Type:          columnType,
//			NotNull:       isNullable == "NO", // "YES" if NULL values can be stored in the column, "NO" if not.
//			AutoIncrement: strings.Contains(extra, "AUTO_INCREMENT"),
//			PrimaryKey:    columnKey == "PRI", // "column_key" is "PRI" if this column a (or part of) PRIMARY KEY
//			DefaultValue:  defaultValue,
//		}
//		tblSpec.Columns = append(tblSpec.Columns, colSpec)
//	}
//
//	if err := colRS.Err(); err != nil {
//		level.Error(c.logger).Log("msg", "error during iterating over table columns result set", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//
//	idxRS, err := c.dbConnection.QueryContext(ctx, selectIndexNames, schemaName, tableName)
//	if err != nil {
//		level.Error(c.logger).Log("msg", "failed to query table indexes", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//	defer idxRS.Close()
//
//	for idxRS.Next() {
//		var indexName, indexType string
//		var seqInIndex, nonUnique int
//		var columnName, expression, nullable sql.NullString
//		if err := idxRS.Scan(&indexName, &seqInIndex, &columnName, &expression, &nullable, &nonUnique, &indexType); err != nil {
//			level.Error(c.logger).Log("msg", "failed to scan table indexes", "schema", schemaName, "table", tableName, "err", err)
//			return nil, err
//		}
//
//		// mysql docs describe column and expression as mutually exclusive,
//		// but at least one of them must be present.
//		if !columnName.Valid && !expression.Valid {
//			level.Error(c.logger).Log("msg", "index without a column or expression", "schema", schemaName, "table", tableName, "index", indexName)
//			continue
//		}
//
//		// Append column to the last index if it's the same as the previous one (i.e. multi-column index)
//		if nIndexes := len(tblSpec.Indexes); nIndexes > 0 && tblSpec.Indexes[nIndexes-1].Name == indexName {
//			lastIndex := &tblSpec.Indexes[nIndexes-1]
//			if len(lastIndex.Columns)+len(lastIndex.Expressions) != seqInIndex-1 {
//				level.Error(c.logger).Log("msg", "unexpected index ordinal position", "schema", schemaName, "table", tableName, "index", indexName, "seq", seqInIndex, "len_columns", len(lastIndex.Columns), "len_expressions", len(lastIndex.Expressions))
//				continue
//			}
//
//			if columnName.Valid {
//				lastIndex.Columns = append(lastIndex.Columns, columnName.String)
//			} else if expression.Valid {
//				lastIndex.Expressions = append(lastIndex.Expressions, expression.String)
//			}
//		} else {
//			idx := indexSpec{
//				Name:     indexName,
//				Type:     indexType,
//				Unique:   nonUnique == 0,                             // 0 if the index cannot contain duplicates, 1 if it can
//				Nullable: nullable.Valid && nullable.String == "YES", // "YES" if the column may contain NULL values
//			}
//
//			if columnName.Valid {
//				idx.Columns = append(idx.Columns, columnName.String)
//			} else if expression.Valid {
//				idx.Expressions = append(idx.Expressions, expression.String)
//			}
//			tblSpec.Indexes = append(tblSpec.Indexes, idx)
//		}
//	}
//
//	if err := idxRS.Err(); err != nil {
//		level.Error(c.logger).Log("msg", "error during iterating over table indexes result set", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//
//	fkRS, err := c.dbConnection.QueryContext(ctx, selectForeignKeys, schemaName, tableName)
//	if err != nil {
//		level.Error(c.logger).Log("msg", "failed to query table foreign keys", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//	defer fkRS.Close()
//
//	for fkRS.Next() {
//		var constraintName, columnName, referencedTableName, referencedColumnName string
//		if err := fkRS.Scan(&constraintName, &columnName, &referencedTableName, &referencedColumnName); err != nil {
//			level.Error(c.logger).Log("msg", "failed to scan foreign keys", "schema", schemaName, "table", tableName, "err", err)
//			return nil, err
//		}
//
//		tblSpec.ForeignKeys = append(tblSpec.ForeignKeys, foreignKey{
//			Name:                 constraintName,
//			ColumnName:           columnName,
//			ReferencedTableName:  referencedTableName,
//			ReferencedColumnName: referencedColumnName,
//		})
//	}
//
//	if err := fkRS.Err(); err != nil {
//		level.Error(c.logger).Log("msg", "error during iterating over foreign keys result set", "schema", schemaName, "table", tableName, "err", err)
//		return nil, err
//	}
//
//	return tblSpec, nil
//}
