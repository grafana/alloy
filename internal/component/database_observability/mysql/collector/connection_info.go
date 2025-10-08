package collector

import (
	"context"
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

	csp, err := database_observability.PopulateCloudProvider(c.CloudProvider, c.DSN)
	if err != nil {
		return err
	}

	if csp != nil && csp.AWS != nil {
		providerName = "aws"
		providerAccount = csp.AWS.ARN.AccountID
		providerRegion = csp.AWS.ARN.Region
		// We only support RDS database for now. Resource types and ARN formats are documented at: https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonrds.html#amazonrds-resources-for-iam-policies
		if resource := csp.AWS.ARN.Resource; strings.HasPrefix(resource, "db:") {
			dbInstanceIdentifier = strings.TrimPrefix(resource, "db:")
		}
	} else if csp != nil && csp.Azure != nil {
		providerName = "azure"
		dbInstanceIdentifier = csp.Azure.Resource
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
