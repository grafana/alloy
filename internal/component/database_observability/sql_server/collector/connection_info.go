package collector

import (
	"context"
	"database/sql"
	"strings"

	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/database_observability"
)

const ConnectionInfoName = "connection_info"

// EngineName is the value the sql_server component uses to identify itself
// wherever an engine identifier is needed, e.g. the connection_info metric's
// "engine" label and the "engine" Loki label added to every log line.
const EngineName = "sql_server"

// SERVERPROPERTY('EngineEdition') values for the Azure SQL offerings whose
// ProductVersion is a fixed placeholder (12.0.2000.8) rather than a real version.
const (
	engineEditionAzureSQLDatabase        = 5
	engineEditionAzureSQLManagedInstance = 8
)

// ResolveEngineVersion returns the value for the engine_version label. Azure
// SQL Database and Managed Instance are evergreen and expose no real version,
// so they are identified by name. Everything else keeps the numeric
// ProductVersion.
func ResolveEngineVersion(engineEdition sql.NullInt64, productVersion sql.NullString) string {
	if engineEdition.Valid {
		switch engineEdition.Int64 {
		case engineEditionAzureSQLDatabase:
			return "Azure SQL Database"
		case engineEditionAzureSQLManagedInstance:
			return "Azure SQL Managed Instance"
		}
	}
	return productVersion.String
}

type ConnectionInfoArguments struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	CloudProvider *database_observability.CloudProvider
	Pinger        database_observability.Pinger
}

type ConnectionInfo struct {
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	InfoMetric    *prometheus.GaugeVec
	CloudProvider *database_observability.CloudProvider
	pinger        database_observability.Pinger

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
		pinger:        args.Pinger,
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
		engine               = EngineName
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
	} else {
		cfg, err := msdsn.Parse(c.DSN)
		if err != nil {
			return err
		}

		if host := cfg.Host; host != "" {
			if strings.HasSuffix(host, "rds.amazonaws.com") {
				providerName = "aws"
				matches := database_observability.RdsRegex.FindStringSubmatch(host)
				if len(matches) > 3 {
					dbInstanceIdentifier = matches[1]
					providerRegion = matches[3]
				}
			} else if strings.HasSuffix(host, "database.windows.net") {
				providerName = "azure"
				matches := database_observability.AzureSQLServerRegex.FindStringSubmatch(host)
				if len(matches) > 1 {
					dbInstanceIdentifier = matches[1]
				}
			}
		}
	}
	c.running.Store(true)

	labelValues := []string{providerName, providerRegion, providerAccount, dbInstanceIdentifier, engine, c.EngineVersion}
	c.InfoMetric.WithLabelValues(labelValues...).Set(1)

	if c.pinger != nil {
		c.stop = database_observability.RunConnectionInfoMonitor(
			ctx,
			c.pinger,
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
