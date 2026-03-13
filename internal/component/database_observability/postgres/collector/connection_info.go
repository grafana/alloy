package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

const (
	ConnectionInfoName     = "connection_info"
	connectionInfoInterval = 5 * time.Minute
)

var engineVersionRegex = regexp.MustCompile(`(?P<version>^[1-9]+\.[1-9]+)(?P<suffix>.*)?$`)

type ConnectionInfoArguments struct {
	DB            *sql.DB
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	CloudProvider *database_observability.CloudProvider
}

type ConnectionInfo struct {
	dbConnection  *sql.DB
	DSN           string
	Registry      *prometheus.Registry
	EngineVersion string
	InfoMetric    *prometheus.GaugeVec
	CloudProvider *database_observability.CloudProvider

	running *atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewConnectionInfo(args ConnectionInfoArguments) (*ConnectionInfo, error) {
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})

	return &ConnectionInfo{
		dbConnection:  args.DB,
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
	labels, err := c.buildLabels()
	if err != nil {
		return err
	}

	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	c.ctx = ctx
	c.cancel = cancel

	c.ping(ctx, labels)

	go func() {
		defer func() {
			c.Registry.Unregister(c.InfoMetric)
			c.running.Store(false)
		}()

		ticker := time.NewTicker(connectionInfoInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.ping(c.ctx, labels)
			}
		}
	}()

	return nil
}

func (c *ConnectionInfo) buildLabels() (prometheus.Labels, error) {
	var (
		providerName         = "unknown"
		providerRegion       = "unknown"
		providerAccount      = "unknown"
		dbInstanceIdentifier = "unknown"
		engine               = "postgres"
		engineVersion        = "unknown"
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
		parts, err := ParseURL(c.DSN)
		if err != nil {
			return nil, err
		}
		if host, ok := parts["host"]; ok {
			if strings.HasSuffix(host, "rds.amazonaws.com") {
				providerName = "aws"
				matches := database_observability.RdsRegex.FindStringSubmatch(host)
				if len(matches) > 3 {
					dbInstanceIdentifier = matches[1]
					providerRegion = matches[3]
				}
			} else if strings.HasSuffix(host, "postgres.database.azure.com") {
				providerName = "azure"
				matches := database_observability.AzurePostgreSQLRegex.FindStringSubmatch(host)
				if len(matches) > 1 {
					dbInstanceIdentifier = matches[1]
				}
			}
		}
	}

	matches := engineVersionRegex.FindStringSubmatch(c.EngineVersion)
	if len(matches) > 1 {
		engineVersion = matches[1]
	}

	return prometheus.Labels{
		"provider_name":          providerName,
		"provider_region":        providerRegion,
		"provider_account":       providerAccount,
		"db_instance_identifier": dbInstanceIdentifier,
		"engine":                 engine,
		"engine_version":         engineVersion,
	}, nil
}

func (c *ConnectionInfo) ping(ctx context.Context, labels prometheus.Labels) {
	if err := c.dbConnection.PingContext(ctx); err != nil {
		c.Registry.Unregister(c.InfoMetric)
		return
	}
	_ = c.Registry.Register(c.InfoMetric)
	c.InfoMetric.With(labels).Set(1)
}

func (c *ConnectionInfo) Stopped() bool {
	return !c.running.Load()
}

func (c *ConnectionInfo) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}
