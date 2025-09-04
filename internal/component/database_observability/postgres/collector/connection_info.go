package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const ConnectionInfoName = "connection_info"

var (
	rdsRegex   = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)
	azureRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.postgres\.database\.azure\.com`)
)

var engineVersionRegex = regexp.MustCompile(`(?P<version>^[1-9]+\.[1-9]+)(?P<suffix>.*)?$`)

type ConnectionInfoArguments struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	CheckInterval time.Duration
	DB            *sql.DB
}

type ConnectionInfo struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	InfoMetric    *prometheus.GaugeVec
	UpMetric      *prometheus.GaugeVec
	CheckInterval time.Duration
	DB            *sql.DB

	running *atomic.Bool
	cancel  context.CancelFunc
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "db_instance_identifier", "engine", "engine_version"})

	upMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_up",
		Help:      "Database connection successful (1) or failed (0)",
	}, []string{"provider_name", "provider_region", "db_instance_identifier", "engine", "engine_version"})

	args.Registry.MustRegister(infoMetric)
	args.Registry.MustRegister(upMetric)

	return &ConnectionInfo{
		DSN:           args.DSN,
		Registry:      args.Registry,
		EngineVersion: args.EngineVersion,
		InfoMetric:    infoMetric,
		UpMetric:      upMetric,
		CheckInterval: args.CheckInterval,
		DB:            args.DB,
		running:       &atomic.Bool{},
	}, nil
}

func (c *ConnectionInfo) Name() string {
	return ConnectionInfoName
}

func (c *ConnectionInfo) Start(ctx context.Context) error {
	c.running.Store(true)

	var (
		providerName         = "unknown"
		providerRegion       = "unknown"
		dbInstanceIdentifier = "unknown"
		engine               = "postgres"
		engineVersion        = "unknown"
	)

	parts, err := ParseURL(c.DSN)
	if err != nil {
		return err
	}

	if host, ok := parts["host"]; ok {
		if strings.HasSuffix(host, "rds.amazonaws.com") {
			providerName = "aws"
			matches := rdsRegex.FindStringSubmatch(host)
			if len(matches) > 3 {
				dbInstanceIdentifier = matches[1]
				providerRegion = matches[3]
			}
		} else if strings.HasSuffix(host, "postgres.database.azure.com") {
			providerName = "azure"
			matches := azureRegex.FindStringSubmatch(host)
			if len(matches) > 1 {
				dbInstanceIdentifier = matches[1]
			}
		}
	}

	matches := engineVersionRegex.FindStringSubmatch(c.EngineVersion)
	if len(matches) > 1 {
		engineVersion = matches[1]
	}

	c.InfoMetric.WithLabelValues(providerName, providerRegion, dbInstanceIdentifier, engine, engineVersion).Set(1)

	update := func(ctx context.Context) {
		val := 0.0
		if c.DB != nil {
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := c.DB.PingContext(checkCtx); err == nil {
				val = 1.0
			}
		}
		c.UpMetric.WithLabelValues(providerName, providerRegion, dbInstanceIdentifier, engine, engineVersion).Set(val)
	}

	ctx2, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	update(ctx2)

	interval := c.CheckInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx2.Done():
				return
			case <-ticker.C:
				update(ctx2)
			}
		}
	}()

	return nil
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.Registry.Unregister(c.InfoMetric)
	c.Registry.Unregister(c.UpMetric)
	c.running.Store(false)
}
