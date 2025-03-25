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
const (
	selectSetupConsumers = `SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ('events_statements_cpu', 'events_statements_history')`
)

type SetupConsumerArguments struct {
	DB       *sql.DB
	Registry *prometheus.Registry

	Logger log.Logger
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
		Name:      "setup_consumer_enabled",
		Help:      "Whether each performance_schema consumer is enabled (1) or disabled (0)",
	}, []string{"consumer_name"})

	args.Registry.MustRegister(infoMetric)

	return &setupConsumer{
		dbConnection:    args.DB,
		Registry:        args.Registry,
		InfoMetric:      infoMetric,
		running:         &atomic.Bool{},
		logger:          args.Logger,
		collectInterval: 5 * time.Second,
	}, nil
}

func (c *setupConsumer) Name() string {
	return SetupConsumerName
}

func (c *setupConsumer) Start(ctx context.Context) error {
	level.Debug(c.logger).Log("msg", SetupConsumerName+" collector started")
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

type consumer struct {
	name    string
	enabled string
}

func (c *setupConsumer) getSetupConsumers(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectSetupConsumers)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query selectSetupConsumers", "err", err)
		return err
	}
	defer rs.Close()

	c.InfoMetric.Reset()

	consumers := map[string]bool{}
	for rs.Next() {
		var consumer consumer
		if err := rs.Scan(&consumer.name, &consumer.enabled); err != nil {
			return fmt.Errorf("error scanning getSetupConsumers row: %w", err)
		}

		enabled := consumer.enabled == "YES"
		consumers[consumer.name] = enabled

		// Set the metric value (1 for enabled, 0 for disabled)
		switch enabled {
		case true:
			c.InfoMetric.WithLabelValues(consumer.name).Set(1)
		default:
			c.InfoMetric.WithLabelValues(consumer.name).Set(0)
		}
	}

	level.Info(c.logger).Log("msg", "consumers", "consumers", consumers)
	return nil
}
