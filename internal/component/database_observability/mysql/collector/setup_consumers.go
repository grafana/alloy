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
	SetupConsumersCollector = "setup_consumers"
)

type SetupConsumersArguments struct {
	DB       *sql.DB
	Registry *prometheus.Registry

	Logger          log.Logger
	CollectInterval time.Duration
}

type SetupConsumers struct {
	dbConnection         *sql.DB
	registry             *prometheus.Registry
	collectInterval      time.Duration
	setupConsumersMetric *prometheus.GaugeVec

	logger  log.Logger
	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSetupConsumers(args SetupConsumersArguments) (*SetupConsumers, error) {
	setupConsumerMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "setup_consumers_enabled",
		Help:      "Whether each performance_schema consumer is enabled (1) or disabled (0)",
	}, []string{"consumer_name"})

	args.Registry.MustRegister(setupConsumerMetric)

	return &SetupConsumers{
		dbConnection:         args.DB,
		registry:             args.Registry,
		setupConsumersMetric: setupConsumerMetric,
		running:              &atomic.Bool{},
		logger:               log.With(args.Logger, "collector", SetupConsumersCollector),
		collectInterval:      args.CollectInterval,
	}, nil
}

func (c *SetupConsumers) Name() string {
	return SetupConsumersCollector
}

func (c *SetupConsumers) Start(ctx context.Context) error {
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
			if err := c.getSetupConsumers(c.ctx); err != nil {
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

func (c *SetupConsumers) Stopped() bool {
	return !c.running.Load()
}

func (c *SetupConsumers) Stop() {
	c.cancel()
	c.registry.Unregister(c.setupConsumersMetric)
	c.running.Store(false)
}

type consumer struct {
	name    string
	enabled string
}

const (
	selectSetupConsumers = `SELECT NAME, ENABLED FROM performance_schema.setup_consumers`
)

func (c *SetupConsumers) getSetupConsumers(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectSetupConsumers)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query selectSetupConsumers", "err", err)
		return err
	}
	defer rs.Close()

	for rs.Next() {
		var consumer consumer
		if err := rs.Scan(&consumer.name, &consumer.enabled); err != nil {
			return fmt.Errorf("failed to scan getSetupConsumers row: %w", err)
		}

		if consumer.enabled == "YES" {
			c.setupConsumersMetric.WithLabelValues(consumer.name).Set(1)
		} else {
			c.setupConsumersMetric.WithLabelValues(consumer.name).Set(0)
		}
	}

	return nil
}
