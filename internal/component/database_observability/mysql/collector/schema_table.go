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
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	OP_SCHEMA_DETECTION = "schema_detection"
	OP_TABLE_DETECTION  = "table_detection"
	OP_CREATE_STATEMENT = "create_statement"
	SchemaTableName     = "schema_table"
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
		TABLE_TYPE,
		CREATE_TIME,
		ifnull(UPDATE_TIME, CREATE_TIME) AS UPDATE_TIME
	FROM
		information_schema.tables
	WHERE
		TABLE_SCHEMA = ?`

	// Note that the fully qualified table name is appendend to the query,
	// for some reason it doesn't work with placeholders.
	showCreateTable = `SHOW CREATE TABLE`

	selectColumnNames = `
	SELECT
		COLUMN_NAME,
		COLUMN_DEFAULT,
		IS_NULLABLE,
		COLUMN_TYPE,
		COLUMN_KEY,
		EXTRA
	FROM
		information_schema.columns
	WHERE
		TABLE_SCHEMA = ? AND TABLE_NAME = ?
	ORDER BY ORDINAL_POSITION ASC`
)

type SchemaTableArguments struct {
	DB              *sql.DB
	InstanceKey     string
	CollectInterval time.Duration
	EntryHandler    loki.EntryHandler
	CacheTTL        time.Duration

	Logger log.Logger
}

type SchemaTable struct {
	dbConnection    *sql.DB
	instanceKey     string
	collectInterval time.Duration
	entryHandler    loki.EntryHandler
	// Cache of table definitions. Entries are removed after a configurable TTL.
	// Key is a string of the form "schema.table@timestamp", where timestamp is
	// the last update time of the table (this allows capturing schema changes
	// at each scan, regardless of caching).
	// TODO(cristian): allow configuring cache size (currently unlimited).
	cache *expirable.LRU[string, tableInfo]

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

type tableInfo struct {
	schema     string
	tableName  string
	tableType  string
	createTime time.Time
	updateTime time.Time
	createStmt string
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

func NewSchemaTable(args SchemaTableArguments) (*SchemaTable, error) {
	return &SchemaTable{
		dbConnection:    args.DB,
		instanceKey:     args.InstanceKey,
		collectInterval: args.CollectInterval,
		entryHandler:    args.EntryHandler,
		cache:           expirable.NewLRU[string, tableInfo](0, nil, args.CacheTTL),
		logger:          log.With(args.Logger, "collector", "SchemaTable"),
		running:         &atomic.Bool{},
	}, nil
}

func (c *SchemaTable) Name() string {
	return SchemaTableName
}

func (c *SchemaTable) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "SchemaTable collector started")

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

func (c *SchemaTable) extractSchema(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectSchemaName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query schemata", "err", err)
		return err
	}
	defer rs.Close()

	var schemas []string
	for rs.Next() {
		var schema string
		if err := rs.Scan(&schema); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan schemata", "err", err)
			break
		}
		schemas = append(schemas, schema)

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": database_observability.JobName},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line:      fmt.Sprintf(`level=info msg="schema detected" op="%s" instance="%s" schema="%s"`, OP_SCHEMA_DETECTION, c.instanceKey, schema),
			},
		}
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over schemas result set", "err", err)
		return err
	}

	if len(schemas) == 0 {
		level.Info(c.logger).Log("msg", "no schema detected from information_schema.schemata")
		return nil
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
			var tableName, tableType string
			var createTime, updateTime time.Time
			if err := rs.Scan(&tableName, &tableType, &createTime, &updateTime); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan tables", "err", err)
				break
			}
			tables = append(tables, tableInfo{
				schema:     schema,
				tableName:  tableName,
				tableType:  tableType,
				createTime: createTime,
				updateTime: updateTime,
			})

			c.entryHandler.Chan() <- loki.Entry{
				Labels: model.LabelSet{"job": database_observability.JobName},
				Entry: logproto.Entry{
					Timestamp: time.Unix(0, time.Now().UnixNano()),
					Line:      fmt.Sprintf(`level=info msg="table detected" op="%s" instance="%s" schema="%s" table="%s"`, OP_TABLE_DETECTION, c.instanceKey, schema, tableName),
				},
			}
		}

		if err := rs.Err(); err != nil {
			level.Error(c.logger).Log("msg", "error during iterating over tables result set", "err", err)
			return err
		}
	}

	if len(tables) == 0 {
		level.Info(c.logger).Log("msg", "no tables detected from information_schema.tables")
		return nil
	}

	// TODO(cristian): consider moving this into the loop above
	for _, table := range tables {
		logKVs := []any{"schema", table.schema, "table", table.tableName}

		fullyQualifiedTable := fmt.Sprintf("%s.%s", table.schema, table.tableName)
		cacheKey := fmt.Sprintf("%s@%d", fullyQualifiedTable, table.updateTime.Unix())

		if c.cache.Contains(cacheKey) {
			level.Debug(c.logger).Log("msg", "table definition already in cache", logKVs)
			continue
		}

		row := c.dbConnection.QueryRowContext(ctx, showCreateTable+" "+fullyQualifiedTable)
		if row.Err() != nil {
			level.Error(c.logger).Log("msg", "failed to show create table", append(logKVs, "err", row.Err()))
			break
		}

		var tableName string
		var createStmt string
		var characterSetClient string
		var collationConnection string
		if table.tableType == "BASE TABLE" {
			if err = row.Scan(&tableName, &createStmt); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan create table", append(logKVs, "err", err))
				break
			}
		} else if table.tableType == "VIEW" {
			if err = row.Scan(&tableName, &createStmt, &characterSetClient, &collationConnection); err != nil {
				level.Error(c.logger).Log("msg", "failed to scan create view", append(logKVs, "err", err))
				break
			}
		}

		table.createStmt = createStmt
		c.cache.Add(cacheKey, table)

		spec, err := c.analyzeTableSpec(ctx, table.schema, table.tableName)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to analyze table spec", append(logKVs, "err", err))
			break
		}
		jsonSpec, err := json.Marshal(spec)
		if err != nil {
			level.Error(c.logger).Log("msg", "failed to marshal table spec", append(logKVs, "err", err))
			break
		}

		c.entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"job": database_observability.JobName},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line: fmt.Sprintf(
					`level=info msg="create table" op="%s" instance="%s" schema="%s" table="%s" create_statement="%s" table_spec="%s"`,
					OP_CREATE_STATEMENT, c.instanceKey, table.schema, table.tableName, base64.StdEncoding.EncodeToString([]byte(createStmt)), base64.StdEncoding.EncodeToString(jsonSpec),
				),
			},
		}
	}

	return nil
}

func (c *SchemaTable) analyzeTableSpec(ctx context.Context, schemaName string, tableName string) (*tableSpec, error) {
	rs, err := c.dbConnection.QueryContext(ctx, selectColumnNames, schemaName, tableName)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query table columns", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}
	defer rs.Close()

	tblSpec := &tableSpec{Columns: []columnSpec{}}

	for rs.Next() {
		var columnName, isNullable, columnType, columnKey, extra string
		var columnDefault sql.NullString
		if err := rs.Scan(&columnName, &columnDefault, &isNullable, &columnType, &columnKey, &extra); err != nil {
			level.Error(c.logger).Log("msg", "failed to scan table columns", "schema", schemaName, "table", tableName, "err", err)
			return nil, err
		}

		extra = strings.ToUpper(extra)

		notNull := false
		if isNullable == "NO" {
			notNull = true
		}

		autoIncrement := false
		if strings.Contains(extra, "AUTO_INCREMENT") {
			autoIncrement = true
		}

		primaryKey := false
		if columnKey == "PRI" {
			primaryKey = true
		}

		defaultValue := ""
		if columnDefault.Valid {
			defaultValue = columnDefault.String
			if strings.Contains(extra, "ON UPDATE CURRENT_TIMESTAMP") {
				defaultValue += " ON UPDATE CURRENT_TIMESTAMP"
			}
		}

		colSpec := columnSpec{
			Name:          columnName,
			Type:          columnType,
			NotNull:       notNull,
			AutoIncrement: autoIncrement,
			PrimaryKey:    primaryKey,
			DefaultValue:  defaultValue,
		}
		tblSpec.Columns = append(tblSpec.Columns, colSpec)
	}

	if err := rs.Err(); err != nil {
		level.Error(c.logger).Log("msg", "error during iterating over table columns result set", "schema", schemaName, "table", tableName, "err", err)
		return nil, err
	}

	return tblSpec, nil
}
