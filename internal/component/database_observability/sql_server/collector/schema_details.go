package collector

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
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
	OP_CREATE_STATEMENT    = "create_statement"
)

// emitInterval is the minimum amount of time that must elapse between
// successive OP_CREATE_STATEMENT emissions for the same table, regardless of
// the configured collect_interval.
const emitInterval = 30 * time.Minute

const (
	selectDatabasesTemplate = `
		SELECT name, QUOTENAME(name)
		FROM sys.databases
		WHERE state_desc = 'ONLINE'
			AND HAS_DBACCESS(name) = 1
			AND name NOT IN %s
		ORDER BY name`

	selectTablesTemplate = `
		SELECT
			table_catalog,
			table_schema,
			table_name,
			table_type
		FROM information_schema.tables
		WHERE table_schema NOT IN %s
		ORDER BY table_schema, table_name`

	// selectColumnsTemplate returns columns information for every table in
	// the given IN-clause of table names, scoped to a single schema.
	// Note that column_type is emitted as the bare SQL Server type family name
	// (e.g. `varchar`, `int`, `decimal`) without precision/length.
	selectColumnsTemplate = `
		SELECT
			OBJECT_NAME(c.object_id) AS table_name,
			c.name AS column_name,
			TYPE_NAME(c.user_type_id) AS column_type,
			c.is_nullable,
			c.is_identity,
			dc.definition AS default_value,
			CASE WHEN pk_ic.column_id IS NOT NULL THEN 1 ELSE 0 END AS is_primary_key
		FROM sys.columns c
		INNER JOIN sys.objects o ON c.object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		LEFT JOIN sys.default_constraints dc ON c.default_object_id = dc.object_id
		LEFT JOIN sys.indexes pk ON pk.object_id = c.object_id AND pk.is_primary_key = 1
		LEFT JOIN sys.index_columns pk_ic ON pk_ic.object_id = c.object_id AND pk_ic.index_id = pk.index_id AND pk_ic.column_id = c.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(c.object_id) IN %s
		ORDER BY OBJECT_NAME(c.object_id), c.column_id`

	// selectIndexesTemplate returns index information for every table in
	// the IN-clause. Note we filter out:
	// - Heaps (i.type = 0) and unnamed rowstore rows
	// - INCLUDE columns (ic.is_included_column = 1)
	selectIndexesTemplate = `
		SELECT
			OBJECT_NAME(i.object_id) AS table_name,
			i.name AS index_name,
			ic.key_ordinal AS seq_in_index,
			c.name AS column_name,
			i.type_desc AS index_type,
			i.is_unique,
			c.is_nullable
		FROM sys.indexes i
		INNER JOIN sys.objects o ON i.object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id AND ic.is_included_column = 0
		INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(i.object_id) IN %s
			AND i.type > 0
			AND i.name IS NOT NULL
		ORDER BY OBJECT_NAME(i.object_id), i.name, ic.key_ordinal`

	// selectForeignKeysTemplate returns foreign key information for every table in
	// the IN-clause. Ordering by constraint_column_id preserves the correlation
	// between parent and referenced columns in composite foreign keys.
	selectForeignKeysTemplate = `
		SELECT
			OBJECT_NAME(fk.parent_object_id) AS table_name,
			fk.name AS constraint_name,
			pcol.name AS column_name,
			OBJECT_NAME(fk.referenced_object_id) AS referenced_table_name,
			rcol.name AS referenced_column_name
		FROM sys.foreign_keys fk
		INNER JOIN sys.objects o ON fk.parent_object_id = o.object_id
		INNER JOIN sys.schemas s ON o.schema_id = s.schema_id
		INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		INNER JOIN sys.columns pcol ON fkc.parent_object_id = pcol.object_id AND fkc.parent_column_id = pcol.column_id
		INNER JOIN sys.columns rcol ON fkc.referenced_object_id = rcol.object_id AND fkc.referenced_column_id = rcol.column_id
		WHERE s.name = @p1 AND OBJECT_NAME(fk.parent_object_id) IN %s
		ORDER BY OBJECT_NAME(fk.parent_object_id), fk.name, fkc.constraint_column_id`
)

type SchemaDetailsArguments struct {
	DB               *sql.DB
	CollectInterval  time.Duration
	ExcludeSchemas   []string
	ExcludeDatabases []string
	EntryHandler     loki.EntryHandler

	Logger *slog.Logger
}

type SchemaDetails struct {
	dbConnection     *sql.DB
	collectInterval  time.Duration
	excludeSchemas   []string
	excludeDatabases []string
	entryHandler     loki.EntryHandler

	// lastEmittedAt records the wall-clock time at which OP_CREATE_STATEMENT
	// was last emitted for a table. The outer key is the database name and
	// the inner key is "schema.table". Used to throttle logging to at most
	// one per emitInterval per table.
	lastEmittedAt map[string]map[string]time.Time

	// now allows overriding time.Now() in tests
	now func() time.Time

	logger  *slog.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type tableInfo struct {
	database     string
	schema       string
	tableName    string
	tableType    string
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

func NewSchemaDetails(args SchemaDetailsArguments) (*SchemaDetails, error) {
	c := &SchemaDetails{
		dbConnection:     args.DB,
		collectInterval:  args.CollectInterval,
		excludeSchemas:   args.ExcludeSchemas,
		excludeDatabases: args.ExcludeDatabases,
		entryHandler:     args.EntryHandler,
		lastEmittedAt:    map[string]map[string]time.Time{},
		now:              time.Now,
		logger:           args.Logger.With("collector", SchemaDetailsCollector),
		running:          &atomic.Bool{},
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

// databaseName holds the raw name plus its server-quoted identifier form,
// which is safe to splice directly into a USE statement
type databaseName struct {
	name   string
	quoted string
}

func (c *SchemaDetails) extractSchema(ctx context.Context) error {
	databases, err := c.listDatabases(ctx)
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}

	now := c.now()
	discovered := make(map[string]struct{}, len(databases))

	if len(databases) == 0 {
		c.logger.Info("no databases detected")
	} else {
		// Pin one connection for the whole cycle as USE mutates session state
		conn, err := c.dbConnection.Conn(ctx)
		if err != nil {
			return fmt.Errorf("failed to acquire database connection: %w", err)
		}
		defer conn.Close()

		for _, db := range databases {
			discovered[db.name] = struct{}{}
			if _, err := conn.ExecContext(ctx, fmt.Sprintf("USE %s", db.quoted)); err != nil {
				c.logger.Error("failed to switch database context, skipping", "database", db.name, "err", err)
				continue
			}
			if err := c.extractSchemaForDatabase(ctx, conn, db.name, now); err != nil {
				c.logger.Error("failed to extract schema for database", "database", db.name, "err", err)
				continue
			}
		}
	}

	// Drop throttle entries only for databases that discovery no longer
	// returns (dropped, access revoked, newly excluded). Per-table cleanup
	// within a still-present database is handled by extractSchemaForDatabase
	// and is gated on a clean scan so a transient failure does not evict
	// state that would defeat emitInterval on the next successful cycle.
	for db := range c.lastEmittedAt {
		if _, ok := discovered[db]; !ok {
			delete(c.lastEmittedAt, db)
		}
	}

	return nil
}

func (c *SchemaDetails) listDatabases(ctx context.Context) ([]databaseName, error) {
	query := fmt.Sprintf(selectDatabasesTemplate, buildExcludedDatabasesClause(c.excludeDatabases))
	rs, err := c.dbConnection.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var databases []databaseName
	for rs.Next() {
		var name, quoted string
		if err := rs.Scan(&name, &quoted); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		databases = append(databases, databaseName{name: name, quoted: quoted})
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over databases result set: %w", err)
	}
	return databases, nil
}

// extractSchemaForDatabase runs the tables discovery + per-schema bulk
// metadata queries scoped to the caller's current database context.
// This function is responsible for pruning stale throttle entries for
// dbName, but only when the tables scan completed cleanly: a mid-iteration
// scan failure yields a partial view and pruning is skipped so we never
// evict entries for tables we simply did not observe this cycle.
func (c *SchemaDetails) extractSchemaForDatabase(ctx context.Context, conn *sql.Conn, dbName string, now time.Time) error {
	query := fmt.Sprintf(selectTablesTemplate, buildExcludedSchemasClause(c.excludeSchemas))
	rs, err := conn.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rs.Close()

	tables := []*tableInfo{}
	// scanComplete tracks whether we iterated the tables result set without
	// bailing out early. If false we have a partial view and must skip the
	// per-database throttle prune to avoid evicting entries for tables we
	// did not get to scan this round.
	scanComplete := true

	for rs.Next() {
		var database, schema, tableName, tableType string
		if err := rs.Scan(&database, &schema, &tableName, &tableType); err != nil {
			c.logger.Error("failed to scan tables", "database", dbName, "err", err)
			scanComplete = false
			break
		}
		tables = append(tables, &tableInfo{
			database:  database,
			schema:    schema,
			tableName: tableName,
			tableType: tableType,
		})

		c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
			logging.LevelInfo,
			OP_TABLE_DETECTION,
			fmt.Sprintf(`database="%s" schema="%s" table="%s"`, database, schema, tableName),
		)
	}

	if err := rs.Err(); err != nil {
		return fmt.Errorf("failed to iterate over tables result set: %w", err)
	}

	if len(tables) == 0 {
		c.logger.Info("no tables detected from information_schema.tables", "database", dbName)
		if scanComplete {
			c.pruneDatabaseThrottle(dbName, nil)
		}
		return nil
	}

	// Compute the due set: tables that have never emitted OP_CREATE_STATEMENT
	// or whose last emission is older than emitInterval.
	// Group by schema to preserve the iteration order from the tables-list query.
	dueBySchema := map[string][]*tableInfo{}
	dueSchemas := []string{}
	for _, t := range tables {
		if inner := c.lastEmittedAt[dbName]; inner != nil {
			if last, ok := inner[schemaTableKey(t.schema, t.tableName)]; ok && now.Sub(last) < emitInterval {
				continue
			}
		}
		if _, exists := dueBySchema[t.schema]; !exists {
			dueSchemas = append(dueSchemas, t.schema)
		}
		dueBySchema[t.schema] = append(dueBySchema[t.schema], t)
	}

	if len(dueSchemas) == 0 {
		if scanComplete {
			c.pruneDatabaseThrottle(dbName, tables)
		}
		return nil
	}

	for _, schema := range dueSchemas {
		dueTables := dueBySchema[schema]
		tableNames := make([]string, 0, len(dueTables))
		for _, t := range dueTables {
			tableNames = append(tableNames, t.tableName)
		}

		specs, err := c.fetchSchemaSpecs(ctx, conn, schema, tableNames)
		if err != nil {
			c.logger.Error("failed to fetch schema specs", "database", dbName, "schema", schema, "err", err)
			continue
		}

		for _, table := range dueTables {
			spec, ok := specs[table.tableName]
			if !ok {
				// This might happen if a table is dropped between the tables query and the metadata queries.
				c.logger.Warn("no bulk metadata rows for table", "database", table.database, "schema", table.schema, "table", table.tableName)
				continue
			}

			b64Spec, err := encodeTableSpec(spec)
			if err != nil {
				c.logger.Error("failed to marshal table spec", "database", table.database, "schema", table.schema, "table", table.tableName, "err", err)
				continue
			}
			table.b64TableSpec = b64Spec

			c.entryHandler.Chan() <- database_observability.BuildLokiEntry(
				logging.LevelInfo,
				OP_CREATE_STATEMENT,
				fmt.Sprintf(
					`database="%s" schema="%s" table="%s" table_spec="%s"`,
					table.database, table.schema, table.tableName, table.b64TableSpec,
				),
			)
			if c.lastEmittedAt[dbName] == nil {
				c.lastEmittedAt[dbName] = map[string]time.Time{}
			}
			c.lastEmittedAt[dbName][schemaTableKey(table.schema, table.tableName)] = now
		}
	}

	if scanComplete {
		c.pruneDatabaseThrottle(dbName, tables)
	}

	return nil
}

// pruneDatabaseThrottle removes entries in c.lastEmittedAt[dbName] whose
// schema.table key is not present in the given tables. If the resulting
// inner map is empty, the outer entry is also deleted. Must only be called
// after a complete scan of the database's tables.
func (c *SchemaDetails) pruneDatabaseThrottle(dbName string, tables []*tableInfo) {
	if _, ok := c.lastEmittedAt[dbName]; !ok {
		return
	}

	currentKeys := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		currentKeys[schemaTableKey(t.schema, t.tableName)] = struct{}{}
	}
	for k := range c.lastEmittedAt[dbName] {
		if _, ok := currentKeys[k]; !ok {
			delete(c.lastEmittedAt[dbName], k)
		}
	}
	if len(c.lastEmittedAt[dbName]) == 0 {
		delete(c.lastEmittedAt, dbName)
	}
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
// with no rows in any of the three result sets are absent. conn must be the
// pinned connection whose current database contains the target schema.
func (c *SchemaDetails) fetchSchemaSpecs(ctx context.Context, conn *sql.Conn, schema string, tableNames []string) (map[string]*tableSpec, error) {
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

	colRS, err := conn.QueryContext(ctx, fmt.Sprintf(selectColumnsTemplate, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		c.logger.Error("failed to query schema columns", "schema", schema, "err", err)
		return nil, err
	}
	defer colRS.Close()

	for colRS.Next() {
		var tableName, columnName, columnType string
		var isNullable, isIdentity, isPrimaryKey bool
		var defaultValue sql.NullString
		if err := colRS.Scan(&tableName, &columnName, &columnType, &isNullable, &isIdentity, &defaultValue, &isPrimaryKey); err != nil {
			c.logger.Error("failed to scan schema columns", "schema", schema, "err", err)
			return nil, err
		}

		defaultVal := ""
		if defaultValue.Valid {
			defaultVal = defaultValue.String
		}

		spec := specOf(tableName)
		spec.Columns = append(spec.Columns, columnSpec{
			Name:          columnName,
			Type:          columnType,
			NotNull:       !isNullable,
			AutoIncrement: isIdentity,
			PrimaryKey:    isPrimaryKey,
			DefaultValue:  defaultVal,
		})
	}

	if err := colRS.Err(); err != nil {
		c.logger.Error("failed to iterate over schema columns result set", "schema", schema, "err", err)
		return nil, err
	}

	idxRS, err := conn.QueryContext(ctx, fmt.Sprintf(selectIndexesTemplate, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		c.logger.Error("failed to query schema indexes", "schema", schema, "err", err)
		return nil, err
	}
	defer idxRS.Close()

	// Track the table whose last appended index we may merge into. When the
	// table changes we always treat the next index row as a new index, even if
	// the index name happens to match.
	var lastTable string
	for idxRS.Next() {
		var tableName, indexName, columnName, indexType string
		var seqInIndex int
		var isUnique, isNullable bool
		if err := idxRS.Scan(&tableName, &indexName, &seqInIndex, &columnName, &indexType, &isUnique, &isNullable); err != nil {
			c.logger.Error("failed to scan schema indexes", "schema", schema, "err", err)
			return nil, err
		}

		spec := specOf(tableName)

		// Append column to the last index if it's the same as the previous one
		// within the same table (i.e. multi-column index).
		if tableName == lastTable {
			if nIndexes := len(spec.Indexes); nIndexes > 0 && spec.Indexes[nIndexes-1].Name == indexName {
				lastIndex := &spec.Indexes[nIndexes-1]
				if len(lastIndex.Columns) != seqInIndex-1 {
					c.logger.Error("unexpected index ordinal position", "schema", schema, "table", tableName, "index", indexName, "seq", seqInIndex, "len_columns", len(lastIndex.Columns))
					continue
				}
				lastIndex.Columns = append(lastIndex.Columns, columnName)
				lastIndex.Nullable = lastIndex.Nullable || isNullable
				continue
			}
		}

		spec.Indexes = append(spec.Indexes, indexSpec{
			Name:     indexName,
			Type:     indexType,
			Unique:   isUnique,
			Nullable: isNullable,
			Columns:  []string{columnName},
		})
		lastTable = tableName
	}

	if err := idxRS.Err(); err != nil {
		c.logger.Error("failed to iterate over schema indexes result set", "schema", schema, "err", err)
		return nil, err
	}

	fkRS, err := conn.QueryContext(ctx, fmt.Sprintf(selectForeignKeysTemplate, database_observability.BuildExclusionClause(tableNames)), schema)
	if err != nil {
		c.logger.Error("failed to query schema foreign keys", "schema", schema, "err", err)
		return nil, err
	}
	defer fkRS.Close()

	for fkRS.Next() {
		var tableName, constraintName, columnName, referencedTableName, referencedColumnName string
		if err := fkRS.Scan(&tableName, &constraintName, &columnName, &referencedTableName, &referencedColumnName); err != nil {
			c.logger.Error("failed to scan schema foreign keys", "schema", schema, "err", err)
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
		c.logger.Error("failed to iterate over schema foreign keys result set", "schema", schema, "err", err)
		return nil, err
	}

	return specs, nil
}

func schemaTableKey(schema, table string) string {
	return fmt.Sprintf("[%s].[%s]", schema, table)
}
