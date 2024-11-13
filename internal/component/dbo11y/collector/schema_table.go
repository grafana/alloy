package collector

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-kit/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type SchemaTableArguments struct {
	DSN            string
	ScrapeInterval time.Duration
	EntryHandler   loki.EntryHandler

	Logger log.Logger
}

type SchemaTable struct {
	dbConnection   *sql.DB
	scrapeInterval time.Duration
	entryHandler   loki.EntryHandler

	logger log.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSchemaTable(args SchemaTableArguments) (*SchemaTable, error) {
	dbConnection, err := sql.Open("mysql", args.DSN)
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
		logger:         args.Logger,
	}, nil
}

func (c *SchemaTable) Run(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", "SchemaTable component running")

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
	c.entryHandler.Chan() <- loki.Entry{
		Labels: model.LabelSet{"lbl": "val"},
		Entry: logproto.Entry{
			Timestamp: time.Unix(0, time.Now().UnixNano()),
			Line:      "TABLE orders (id INT, customer_id INT, amount DECIMAL(10,2))",
		},
	}
	return nil
}
