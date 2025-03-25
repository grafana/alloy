package collector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	SetupConsumerName = "setup_consumer"
)

type SetupConsumerArguments struct {
	DB       *sql.DB
	Registry *prometheus.Registry

	logger log.Logger
}

type setupConsumer struct {
	dbConnection    *sql.DB
	Registry        *prometheus.Registry
	collectInterval time.Duration
	InfoMetric      *prometheus.GaugeVec

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSetupConsumer(args SetupConsumerArguments) (*setupConsumer, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "setup_consumer_info",
		Help:      "Information about enabled consumers in the performance_schema.setup_consumer table",
	}, []string{"events_statements_cpu", "events_statements_history"})

	args.Registry.MustRegister(infoMetric)

	return &setupConsumer{
		dbConnection: args.DB,
		Registry:     args.Registry,
		InfoMetric:   infoMetric,
		running:      &atomic.Bool{},
		logger:       args.logger,
	}, nil
}

func (c *setupConsumer) Name() string {
	return SetupConsumerName
}

func (c *setupConsumer) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", SchemaTableName+" collector started")
	c.running.Store(true)
	// c.InfoMetric //todo

	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	go func() {
		defer func() {
			c.Stop()
			c.running.Store(false)
			//c.Registry.Unregister(c.InfoMetric)} //todo
		}()

		ticker := time.NewTicker(c.collectInterval)

		for {
			if err := c.getSetupConsumers(c.ctx); err != nil {
				level.Error(c.logger).Log("msg", "setupConsumer collector error", "err", err)
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

func (c *setupConsumer) Stopped() bool {
	return !c.running.Load()
}

func (c *setupConsumer) Stop() {
	c.cancel()
}

func (c *setupConsumer) getSetupConsumers(ctx context.Context) error {
	row := c.dbConnection.QueryRowContext(ctx, selectSchemaName)

	var someStuff string
	if err := row.Scan(&someStuff); err != nil {
		return fmt.Errorf("error scanning getSetupConsumers row: %w", err)
	}

	return nil
}
