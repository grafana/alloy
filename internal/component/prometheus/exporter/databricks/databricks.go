package databricks

import (
	"log/slog"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/databricks-prometheus-exporter/collector"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.databricks",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "databricks"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	cfg := a.toConfig()
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, cfg, a.ServerHostname)
}

// DefaultArguments holds the default settings for the databricks exporter
var DefaultArguments = Arguments{
	QueryTimeout:        5 * time.Minute,
	BillingLookback:     24 * time.Hour,
	JobsLookback:        3 * time.Hour,
	PipelinesLookback:   3 * time.Hour,
	QueriesLookback:     2 * time.Hour,
	SLAThresholdSeconds: 3600,
	CollectTaskRetries:  false,
}

// Arguments controls the databricks exporter.
type Arguments struct {
	ServerHostname      string            `alloy:"server_hostname,attr"`
	WarehouseHTTPPath   string            `alloy:"warehouse_http_path,attr"`
	ClientID            string            `alloy:"client_id,attr"`
	ClientSecret        alloytypes.Secret `alloy:"client_secret,attr"`
	QueryTimeout        time.Duration     `alloy:"query_timeout,attr,optional"`
	BillingLookback     time.Duration     `alloy:"billing_lookback,attr,optional"`
	JobsLookback        time.Duration     `alloy:"jobs_lookback,attr,optional"`
	PipelinesLookback   time.Duration     `alloy:"pipelines_lookback,attr,optional"`
	QueriesLookback     time.Duration     `alloy:"queries_lookback,attr,optional"`
	SLAThresholdSeconds int               `alloy:"sla_threshold_seconds,attr,optional"`
	CollectTaskRetries  bool              `alloy:"collect_task_retries,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) toConfig() *databricksConfig {
	return &databricksConfig{
		serverHostname:      a.ServerHostname,
		warehouseHTTPPath:   a.WarehouseHTTPPath,
		clientID:            a.ClientID,
		clientSecret:        string(a.ClientSecret),
		queryTimeout:        a.QueryTimeout,
		billingLookback:     a.BillingLookback,
		jobsLookback:        a.JobsLookback,
		pipelinesLookback:   a.PipelinesLookback,
		queriesLookback:     a.QueriesLookback,
		slaThresholdSeconds: a.SLAThresholdSeconds,
		collectTaskRetries:  a.CollectTaskRetries,
	}
}

// databricksConfig implements integrations.Config for creating the exporter.
type databricksConfig struct {
	serverHostname      string
	warehouseHTTPPath   string
	clientID            string
	clientSecret        string
	queryTimeout        time.Duration
	billingLookback     time.Duration
	jobsLookback        time.Duration
	pipelinesLookback   time.Duration
	queriesLookback     time.Duration
	slaThresholdSeconds int
	collectTaskRetries  bool
}

// Name returns the name of the integration.
func (c *databricksConfig) Name() string {
	return "databricks"
}

// InstanceKey returns the hostname as the instance identifier.
func (c *databricksConfig) InstanceKey(_ string) (string, error) {
	return c.serverHostname, nil
}

// NewIntegration creates a new databricks integration.
func (c *databricksConfig) NewIntegration(l log.Logger) (integrations.Integration, error) {
	exporterCfg := &collector.Config{
		ServerHostname:      c.serverHostname,
		WarehouseHTTPPath:   c.warehouseHTTPPath,
		ClientID:            c.clientID,
		ClientSecret:        c.clientSecret,
		QueryTimeout:        c.queryTimeout,
		BillingLookback:     c.billingLookback,
		JobsLookback:        c.jobsLookback,
		PipelinesLookback:   c.pipelinesLookback,
		QueriesLookback:     c.queriesLookback,
		SLAThresholdSeconds: c.slaThresholdSeconds,
		CollectTaskRetries:  c.collectTaskRetries,
	}

	if err := exporterCfg.Validate(); err != nil {
		return nil, err
	}

	logger := slog.New(logging.NewSlogGoKitHandler(l))
	col := collector.NewCollector(logger, exporterCfg)
	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(col),
	), nil
}
