package collector

import (
	"context"
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/database_observability"
)

const ConnectionInfoName = "connection_info"

type ConnectionInfoArguments struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	CloudProvider *database_observability.CloudProvider
	DB            *sql.DB
}

type ConnectionInfo struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	InfoMetric    *prometheus.GaugeVec
	CloudProvider *database_observability.CloudProvider
	dbConnection  *sql.DB

	running *atomic.Bool
	stop    func()
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
		dbConnection:  args.DB,
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
		engine               = "mssql"
	)

	if c.CloudProvider != nil {
		if c.CloudProvider.AWS != nil {
			providerName = "aws"
			providerAccount = c.CloudProvider.AWS.ARN.AccountID
			providerRegion = c.CloudProvider.AWS.ARN.Region

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
		if c.CloudProvider.GCP != nil {
			providerName = "gcp"
			providerRegion = c.CloudProvider.GCP.Region
			providerAccount = c.CloudProvider.GCP.ProjectID
			dbInstanceIdentifier = c.CloudProvider.GCP.InstanceID
		}
	}
	c.running.Store(true)

	labelValues := []string{providerName, providerRegion, providerAccount, dbInstanceIdentifier, engine, c.EngineVersion}
	c.InfoMetric.WithLabelValues(labelValues...).Set(1)

	if c.dbConnection != nil {
		c.stop = database_observability.RunConnectionInfoMonitor(
			ctx,
			c.dbConnection,
			c.Registry,
			c.InfoMetric,
			labelValues,
			func() { c.running.Store(false) },
			nil,
		)
	}

	return nil
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	if c.stop != nil {
		c.stop()
	}
	c.Registry.Unregister(c.InfoMetric)
	c.running.Store(false)
}
