package collector

import (
	"context"
	"net"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const ConnectionInfoName = "connection_info"

type ConnectionInfoArguments struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	CloudProvider *database_observability.CloudProvider
}

type ConnectionInfo struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	InfoMetric    *prometheus.GaugeVec
	CloudProvider *database_observability.CloudProvider

	running *atomic.Bool
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})

	args.Registry.MustRegister(infoMetric)

	return &ConnectionInfo{
		DSN:           args.DSN,
		Registry:      args.Registry,
		EngineVersion: args.EngineVersion,
		InfoMetric:    infoMetric,
		CloudProvider: args.CloudProvider,
		running:       &atomic.Bool{},
	}, nil
}

func (c *ConnectionInfo) Name() string {
	return ConnectionInfoName
}

func (c *ConnectionInfo) Start(ctx context.Context) error {
	var (
		providerName         = "unknown"
		providerRegion       = "unknown"
		providerAccount      = "unknown"
		dbInstanceIdentifier = "unknown"
		engine               = "mysql"
	)

	if c.CloudProvider != nil {
		if c.CloudProvider.AWS != nil {
			providerName = "aws"
			providerAccount = c.CloudProvider.AWS.ARN.AccountID
			providerRegion = c.CloudProvider.AWS.ARN.Region

			// We only support RDS database for now. Resource types and ARN formats are documented at: https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonrds.html#amazonrds-resources-for-iam-policies
			if resource := c.CloudProvider.AWS.ARN.Resource; strings.HasPrefix(resource, "db:") {
				dbInstanceIdentifier = strings.TrimPrefix(resource, "db:")
			}
		}
		if c.CloudProvider.Azure != nil {
			providerName = "azure"
			dbInstanceIdentifier = c.CloudProvider.Azure.ServerName
			providerRegion = c.CloudProvider.Azure.ResourceGroup
			providerAccount = c.CloudProvider.Azure.SubscriptionID
		}
	} else {
		cfg, err := mysql.ParseDSN(c.DSN)
		if err != nil {
			return err
		}

		host, _, err := net.SplitHostPort(cfg.Addr)
		if err == nil && host != "" {
			if strings.HasSuffix(host, "rds.amazonaws.com") {
				providerName = "aws"
				matches := database_observability.RdsRegex.FindStringSubmatch(host)
				if len(matches) > 3 {
					dbInstanceIdentifier = matches[1]
					providerRegion = matches[3]
				}
			} else if strings.HasSuffix(host, "mysql.database.azure.com") {
				providerName = "azure"
				matches := database_observability.AzureMySQLRegex.FindStringSubmatch(host)
				if len(matches) > 1 {
					dbInstanceIdentifier = matches[1]
				}
			}
		}
	}
	c.running.Store(true)

	c.InfoMetric.WithLabelValues(providerName, providerRegion, providerAccount, dbInstanceIdentifier, engine, c.EngineVersion).Set(1)
	return nil
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	c.Registry.Unregister(c.InfoMetric)
	c.running.Store(false)
}
