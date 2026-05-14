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
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	SchemaDetailsCollector = "schema_details"
	OP_TABLE_DETECTION     = "table_detection"
	OP_CREATE_STATEMENT    = "create_statement"
)

// EmitInterval is the minimum amount of time that must elapse between
// successive OP_CREATE_STATEMENT emissions for the same table, regardless of
// the configured collect_interval.
const EmitInterval = 30 * time.Minute

const (
	selectTablesTemplate = `
	SELECT
		TABLE_SCHEMA,
		TABLE_NAME,
		TABLE_TYPE,
		CREATE_TIME,
		IFNULL(UPDATE_TIME, CREATE_TIME) AS UPDATE_TIME
	FROM
		information_schema.tables
	WHERE
		TABLE_SCHEMA NOT IN %s
	ORDER BY TABLE_SCHEMA, TABLE_NAME`

	// Note that the fully qualified table name is appendend to the query,
	// we can't use placeholders with this statement.
	showCreateTable = `SHOW CREATE TABLE`

	selectColumnNames = `
	SELECT
		TABLE_NAME,
		COLUMN_NAME,
		COLUMN_DEFAULT,
		IS_NULLABLE,
		COLUMN_TYPE,
		COLUMN_KEY,
		EXTRA
	FROM
		information_schema.columns
	WHERE
		TABLE_SCHEMA = ? AND TABLE_NAME IN %s
	ORDER BY TABLE_NAME, ORDINAL_POSITION ASC`

	selectIndexNames = `
		SELECT
			table_name,
			index_name,
			seq_in_index,
			column_name,
			expression,
			nullable,
			non_unique,
			index_type
		FROM
			information_schema.statistics
		WHERE
			table_schema = ? AND table_name IN %s
		ORDER BY table_name, index_name, seq_in_index`

	// Ignore 'PRIMARY' constraints, as they're already covered by the query above
	selectForeignKeys = `
		SELECT
			table_name,
			constraint_name,
			column_name,
			referenced_table_name,
			referenced_column_name
		FROM
			information_schema.key_column_usage
		WHERE
			constraint_name <> 'PRIMARY'
			AND referenced_table_schema is not null
			AND table_schema = ? AND table_name IN %s
		ORDER BY table_name, constraint_name, ordinal_position`
)

type SchemaDetailsArguments struct {
	DB              *sql.DB
	CollectInterval time.Duration
	ExcludeSchemas  []string
	EntryHandler    loki.EntryHandler

	Logger log.Logger
}

type SchemaDetails struct {
	dbConnection    *sql.DB
	collectInterval time.Duration
	excludeSchemas  []string
	entryHandler    loki.EntryHandler

	// lastEmittedAt records the wall-clock time at which OP_CREATE_STATEMENT
	// was last emitted for a "schema.table" key. Used to throttle logging
	// to at most one per EmitInterval per table.
	lastEmittedAt map[string]time.Time

	// nowFn allows overriding time.Now() in tests
	nowFn func() time.Time

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
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

func NewSchemaDetails(args SchemaDetailsArguments) (*SchemaDetails, error) {
	c := &SchemaDetails{
		dbConnection:    args.DB,
		collectInterval: args.CollectInterval,
		excludeSchemas:  args.ExcludeSchemas,
		entryHandler:    args.EntryHandler,
		lastEmittedAt:   map[string]time.Time{},
		nowFn:           time.Now,
		logger:          log.With(args.Logger, "collector", SchemaDetailsCollector),
		running:         &atomic.Bool{},
	}

	return c, nil
}

func (c *SchemaDetails) Name() string {
	return SchemaDetailsCollector
}

func (c *SchemaDetails) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "collector started")

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
				level.Error(c.logger).Log("msg", "collector error", "err", err)
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

	now := c.nowFn()
	tables := []*tableInfo{}
	seenTables := map[string]struct{}{}
	for rs.Next() {
		var schema, tableName, tableType string
		var createTime, updateTime time.Time
		if err := rs.Scan(&schema, &tableName, &tableType, &createTime, &updateTime); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan tables", "err", err)
			break
		}
		tables = append(tables, &tableInfo{
			schema:        schema,
			tableName:     tableName,
			tableType:     tableType,
			createTime:    createTime,
			updateTime:    updateTime,
			b64CreateStmt: "",
			b64TableSpec:  "",
		})
		seenTables[fullyQualifiedName(schema, tableName)] = struct{}{}

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_TABLE_DETECTION,
			fmt.Sprintf(`schema="%s" table="%s"`, schema, tableName),
		)
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over tables result set: %w", err)
	}

	// Cleanup: drop throttle entries for tables that no longer exist (e.g.
	// tables dropped or renamed). Done before the empty-tables early return so
	// the map is also pruned when every monitored table disappears.
	for k := range c.lastEmittedAt {
		if _, ok := seenTables[k]; !ok {
			delete(c.lastEmittedAt, k)
		}
	}

	if len(tables) == 0 {
		level.Info(c.logger).Log("msg", "no tables detected from information_schema.tables")
		return nil
	}

	// Compute the due set: tables that have never emitted OP_CREATE_STATEMENT
	// or whose last emission is older than EmitInterval.
	// Group by schema to preserve the iteration order from the tables-list query.
	dueBySchema := map[string][]*tableInfo{}
	dueSchemas := []string{}
	for _, t := range tables {
		k := fullyQualifiedName(t.schema, t.tableName)
		if last, ok := c.lastEmittedAt[k]; ok && now.Sub(last) < EmitInterval {
			continue
		}
		if _, exists := dueBySchema[t.schema]; !exists {
			dueSchemas = append(dueSchemas, t.schema)
		}
		dueBySchema[t.schema] = append(dueBySchema[t.schema], t)
	}

	if len(dueSchemas) == 0 {
		return nil
	}

	for _, schema := range dueSchemas {
		dueTables := dueBySchema[schema]
		tableNames := make([]string, 0, len(dueTables))
		for _, t := range dueTables {
			tableNames = append(tableNames, t.tableName)
		}

		specs, err := c.fetchSchemaSpecs(ctx, schema, tableNames)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to fetch schema specs", "schema", schema, "err", err)
			continue
		}

		for _, table := range dueTables {
			fullyQualifiedTable := fullyQualifiedName(table.schema, table.tableName)

			if err := c.fetchCreateStatement(ctx, fullyQualifiedTable, table); err != nil {
				level.Error(c.logger).Log("msg", "failed to get table create statement", "schema", table.schema, "table", table.tableName, "err", err)
				continue
			}

			spec, ok := specs[table.tableName]
			if !ok {
				// This might happen if a table is dropped between the tables query and the metadata queries.
				level.Warn(c.logger).Log("msg", "no bulk metadata rows for table", "schema", table.schema, "table", table.tableName)
				continue
			}

			b64Spec, err := encodeTableSpec(spec)
			if err != nil {
				level.Error(c.logger).Log("msg", "failed to marshal table spec", "schema", table.schema, "table", table.tableName, "err", err)
				continue
			}
			table.b64TableSpec = b64Spec

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_CREATE_STATEMENT,
				fmt.Sprintf(
					`schema="%s" table="%s" create_statement="%s" table_spec="%s"`,
					table.schema, table.tableName, table.b64CreateStmt, table.b64TableSpec,
				),
			)
			c.lastEmittedAt[fullyQualifiedName(table.schema, table.tableName)] = now
		}
	}

	return nil
}

// fetchCreateStatement runs SHOW CREATE TABLE for a single table and stores the
// base64-encoded DDL on the provided tableInfo.
func (c *SchemaDetails) fetchCreateStatement(ctx context.Context, fullyQualifiedTable string, table *tableInfo) error {
	row := c.dbConnection.QueryRowContext(ctx, showCreateTable+" "+fullyQualifiedTable)
	if err := row.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to show create table", "schema", table.schema, "table", table.tableName, "err", err)
		return err
	}

	var tableName, createStmt, characterSetClient, collationConnection string
	switch table.tableType {
	case "BASE TABLE":
		if err := row.Scan(&tableName, &createStmt); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan create table", "schema", table.schema, "table", table.tableName, "err", err)
			return err
		}
	case "VIEW":
		if err := row.Scan(&tableName, &createStmt, &characterSetClient, &collationConnection); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan create view", "schema", table.schema, "table", table.tableName, "err", err)
			return err
		}
	default:
		level.Error(c.logger).Log("msg", "unknown table type", "schema", table.schema, "table", table.tableName, "table_type", table.tableType)
		return fmt.Errorf("unknown table type: %s", table.tableType)
	}
	table.b64CreateStmt = base64.StdEncoding.EncodeToString([]byte(createStmt))
	return nil
}

// encodeTableSpec serializes a tableSpec to base64-encoded JSON.
func encodeTableSpec(spec *tableSpec) (string, error) {
	jsonSpec, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(jsonSpec), nil
}

// fetchSchemaSpecs runs three queries scoped to (schema, tableNames) to fetch
// columns, indexes and foreign keys for the given tables in bulk and groups
// the rows by table name. The returned map is keyed by table name; tables
// with no rows in any of the three result sets are absent.
func (c *SchemaDetails) fetchSchemaSpecs(ctx context.Context, schema string, tableNames []string) (map[string]*tableSpec, error) {
	specs := map[string]*tableSpec{}
	if len(tableNames) == 0 {
		return specs, nil
	}

	specOf := func(name string) *tableSpec {
		s, ok := specs[name]
		if !ok {
			s = &tableSpec{Columns: []columnSpec{}}
			specs[name] = s
		}
		return s
	}

	colRS, err := c.dbConnection.QueryContext(ctx, fmt.Sprintf(selectColumnNames, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schema columns", "schema", schema, "err", err)
		return nil, err
	}
	defer colRS.Close()

	for colRS.Next() {
		var tableName, columnName, isNullable, columnType, columnKey, extra string
		var columnDefault sql.NullString
		if err := colRS.Scan(&tableName, &columnName, &columnDefault, &isNullable, &columnType, &columnKey, &extra); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schema columns", "schema", schema, "err", err)
			return nil, err
		}

		extra = strings.ToUpper(extra) // "extra" might contain a variety of textual information
		defaultValue := ""
		if columnDefault.Valid {
			defaultValue = columnDefault.String
			if strings.Contains(extra, "ON UPDATE CURRENT_TIMESTAMP") {
				defaultValue += " ON UPDATE CURRENT_TIMESTAMP"
			}
		}

		spec := specOf(tableName)
		spec.Columns = append(spec.Columns, columnSpec{
			Name:          columnName,
			Type:          columnType,
			NotNull:       isNullable == "NO", // "YES" if NULL values can be stored in the column, "NO" if not.
			AutoIncrement: strings.Contains(extra, "AUTO_INCREMENT"),
			PrimaryKey:    columnKey == "PRI", // "column_key" is "PRI" if this column a (or part of) PRIMARY KEY
			DefaultValue:  defaultValue,
		})
	}

	if err := colRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over schema columns result set", "schema", schema, "err", err)
		return nil, err
	}

	idxRS, err := c.dbConnection.QueryContext(ctx, fmt.Sprintf(selectIndexNames, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schema indexes", "schema", schema, "err", err)
		return nil, err
	}
	defer idxRS.Close()

	// Track the table whose last appended index we may merge into. When the
	// table changes we always treat the next index row as a new index, even if
	// the index name happens to match.
	var lastTable string
	for idxRS.Next() {
		var tableName, indexName, indexType string
		var seqInIndex, nonUnique int
		var columnName, expression, nullable sql.NullString
		if err := idxRS.Scan(&tableName, &indexName, &seqInIndex, &columnName, &expression, &nullable, &nonUnique, &indexType); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schema indexes", "schema", schema, "err", err)
			return nil, err
		}

		// mysql docs describe column and expression as mutually exclusive,
		// but at least one of them must be present.
		if !columnName.Valid && !expression.Valid {
			level.Error(c.logger).Log("msg", "index without a column or expression", "schema", schema, "table", tableName, "index", indexName)
			continue
		}

		spec := specOf(tableName)

		// Append column to the last index if it's the same as the previous one
		// within the same table (i.e. multi-column index).
		if tableName == lastTable {
			if nIndexes := len(spec.Indexes); nIndexes > 0 && spec.Indexes[nIndexes-1].Name == indexName {
				lastIndex := &spec.Indexes[nIndexes-1]
				if len(lastIndex.Columns)+len(lastIndex.Expressions) != seqInIndex-1 {
					level.Error(c.logger).Log("msg", "unexpected index ordinal position", "schema", schema, "table", tableName, "index", indexName, "seq", seqInIndex, "len_columns", len(lastIndex.Columns), "len_expressions", len(lastIndex.Expressions))
					continue
				}

				if columnName.Valid {
					lastIndex.Columns = append(lastIndex.Columns, columnName.String)
				} else if expression.Valid {
					lastIndex.Expressions = append(lastIndex.Expressions, expression.String)
				}
				continue
			}
		}

		idx := indexSpec{
			Name:     indexName,
			Type:     indexType,
			Unique:   nonUnique == 0,                             // 0 if the index cannot contain duplicates, 1 if it can
			Nullable: nullable.Valid && nullable.String == "YES", // "YES" if the column may contain NULL values
		}

		if columnName.Valid {
			idx.Columns = append(idx.Columns, columnName.String)
		} else if expression.Valid {
			idx.Expressions = append(idx.Expressions, expression.String)
		}
		spec.Indexes = append(spec.Indexes, idx)
		lastTable = tableName
	}

	if err := idxRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over schema indexes result set", "schema", schema, "err", err)
		return nil, err
	}

	fkRS, err := c.dbConnection.QueryContext(ctx, fmt.Sprintf(selectForeignKeys, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schema foreign keys", "schema", schema, "err", err)
		return nil, err
	}
	defer fkRS.Close()

	for fkRS.Next() {
		var tableName, constraintName, columnName, referencedTableName, referencedColumnName string
		if err := fkRS.Scan(&tableName, &constraintName, &columnName, &referencedTableName, &referencedColumnName); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schema foreign keys", "schema", schema, "err", err)
			return nil, err
		}

		spec := specOf(tableName)
		spec.ForeignKeys = append(spec.ForeignKeys, foreignKey{
			Name:                 constraintName,
			ColumnName:           columnName,
			ReferencedTableName:  referencedTableName,
			ReferencedColumnName: referencedColumnName,
		})
	}

	if err := fkRS.Err(); err != nil {
		level.Error(c.logger).Log("msg", "failed to iterate over schema foreign keys result set", "schema", schema, "err", err)
		return nil, err
	}

	return specs, nil
}

func fullyQualifiedName(schema, table string) string {
	return fmt.Sprintf("`%s`.`%s`", schema, table)
}
