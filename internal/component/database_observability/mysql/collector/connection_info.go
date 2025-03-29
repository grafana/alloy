package collector

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const ConnectionInfoName = "connection_info"

var rdsRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)

type ConnectionInfoArguments struct {
	DSN            string
	Registry       *prometheus.Registry
	DB             *sql.DB
	Logger         log.Logger
	ScrapeInterval time.Duration
}

type ConnectionInfo struct {
	DSN                 string
	dbConnection        *sql.DB
	Registry            *prometheus.Registry
	InfoMetric          *prometheus.GaugeVec
	SetupConsumerMetric *prometheus.GaugeVec
	collectInterval     time.Duration

	running *atomic.Bool
	logger  log.Logger
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "db_instance_identifier"})

	setupConsumerMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "setup_consumer_enabled",
		Help:      "Whether each performance_schema consumer is enabled (1) or disabled (0)",
	}, []string{"consumer_name"})

	args.Registry.MustRegister(infoMetric)
	args.Registry.MustRegister(setupConsumerMetric)

	return &ConnectionInfo{
		DSN:                 args.DSN,
		dbConnection:        args.DB,
		Registry:            args.Registry,
		InfoMetric:          infoMetric,
		SetupConsumerMetric: setupConsumerMetric,
		collectInterval:     args.ScrapeInterval,
		running:             &atomic.Bool{},
		logger:              args.Logger,
	}, nil
}

func (c *ConnectionInfo) Name() string {
	return ConnectionInfoName
}

func (c *ConnectionInfo) Start(ctx context.Context) error {
	cfg, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		return err
	}

	c.running.Store(true)
	c.getProviderAndInstanceInfo(err, cfg)
	c.runGetSetupConsumers(ctx)

	return nil
}

func (c *ConnectionInfo) getProviderAndInstanceInfo(err error, cfg *mysql.Config) {
	var (
		providerName         = "unknown"
		providerRegion       = "unknown"
		dbInstanceIdentifier = "unknown"
	)

	host, _, err := net.SplitHostPort(cfg.Addr)
	if err == nil && host != "" {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			providerName = "aws"
			matches := rdsRegex.FindStringSubmatch(host)
			if len(matches) > 3 {
				dbInstanceIdentifier = matches[1]
				providerRegion = matches[3]
			}
		}
	}

	c.InfoMetric.WithLabelValues(providerName, providerRegion, dbInstanceIdentifier).Set(1)
}

func (c *ConnectionInfo) runGetSetupConsumers(ctx context.Context) {
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
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	c.cancel()
	c.Registry.Unregister(c.InfoMetric)
	c.Registry.Unregister(c.SetupConsumerMetric)
	c.running.Store(false)
}

type consumer struct {
	name    string
	enabled string
}

const (
	selectSetupConsumers = `SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ('events_statements_cpu', 'events_statements_history')`
)

func (c *ConnectionInfo) getSetupConsumers(ctx context.Context) error {
	rs, err := c.dbConnection.QueryContext(ctx, selectSetupConsumers)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to query selectSetupConsumers", "err", err)
		return err
	}
	defer rs.Close()

	c.SetupConsumerMetric.Reset()

	for rs.Next() {
		var consumer consumer
		if err := rs.Scan(&consumer.name, &consumer.enabled); err != nil {
			return fmt.Errorf("error scanning getSetupConsumers row: %w", err)
		}

		enabled := consumer.enabled == "YES"
		switch enabled {
		case true:
			c.SetupConsumerMetric.WithLabelValues(consumer.name).Set(1)
		default:
			c.SetupConsumerMetric.WithLabelValues(consumer.name).Set(0)
		}
	}

	return nil
}
