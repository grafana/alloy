package collector

import (
	"context"
	"regexp"
	"strings"

	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const ConnectionInfoName = "connection_info"

var (
	rdsRegex   = regexp.MustCompile(`(?P<identifier>[^\.]+)\.([^\.]+)\.(?P<region>[^\.]+)\.rds\.amazonaws\.com`)
	azureRegex = regexp.MustCompile(`(?P<identifier>[^\.]+)\.postgres\.database\.azure\.com`)
)

type ConnectionInfoArguments struct {
	DSN      string
	Registry *prometheus.Registry
}

type ConnectionInfo struct {
	DSN        string
	Registry   *prometheus.Registry
	InfoMetric *prometheus.GaugeVec

	running *atomic.Bool
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "db_instance_identifier", "engine"})

	util.MustRegisterOrGet(args.Registry, infoMetric)

	return &ConnectionInfo{
		DSN:        args.DSN,
		Registry:   args.Registry,
		InfoMetric: infoMetric,
		running:    &atomic.Bool{},
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
	c.InfoMetric.WithLabelValues(providerName, providerRegion, dbInstanceIdentifier, engine).Set(1)
	return nil
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	c.Registry.Unregister(c.InfoMetric)
	c.running.Store(false)
}
