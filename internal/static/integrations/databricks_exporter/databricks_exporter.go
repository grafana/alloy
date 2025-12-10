package databricks_exporter

import (
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/databricks-prometheus-exporter/collector"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig is the default config for the databricks integration
var DefaultConfig = Config{
	QueryTimeout:        5 * time.Minute,
	BillingLookback:     24 * time.Hour,
	JobsLookback:        2 * time.Hour,
	PipelinesLookback:   2 * time.Hour,
	QueriesLookback:     1 * time.Hour,
	SLAThresholdSeconds: 3600,
	CollectTaskRetries:  false,
}

// Config is the configuration for the databricks integration
type Config struct {
	ServerHostname      string             `yaml:"server_hostname,omitempty"`
	WarehouseHTTPPath   string             `yaml:"warehouse_http_path,omitempty"`
	ClientID            string             `yaml:"client_id,omitempty"`
	ClientSecret        config_util.Secret `yaml:"client_secret,omitempty"`
	QueryTimeout        time.Duration      `yaml:"query_timeout,omitempty"`
	BillingLookback     time.Duration      `yaml:"billing_lookback,omitempty"`
	JobsLookback        time.Duration      `yaml:"jobs_lookback,omitempty"`
	PipelinesLookback   time.Duration      `yaml:"pipelines_lookback,omitempty"`
	QueriesLookback     time.Duration      `yaml:"queries_lookback,omitempty"`
	SLAThresholdSeconds int                `yaml:"sla_threshold_seconds,omitempty"`
	CollectTaskRetries  bool               `yaml:"collect_task_retries,omitempty"`
}

func (c *Config) exporterConfig() *collector.Config {
	return &collector.Config{
		ServerHostname:      c.ServerHostname,
		WarehouseHTTPPath:   c.WarehouseHTTPPath,
		ClientID:            c.ClientID,
		ClientSecret:        string(c.ClientSecret),
		QueryTimeout:        c.QueryTimeout,
		BillingLookback:     c.BillingLookback,
		JobsLookback:        c.JobsLookback,
		PipelinesLookback:   c.PipelinesLookback,
		QueriesLookback:     c.QueriesLookback,
		SLAThresholdSeconds: c.SLAThresholdSeconds,
		CollectTaskRetries:  c.CollectTaskRetries,
	}
}

func (c *Config) InstanceKey(_ string) (string, error) {
	return c.ServerHostname, nil
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "databricks"
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("databricks"))
}

// NewIntegration creates a new integration from the config.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	exporterConfig := c.exporterConfig()

	if err := exporterConfig.Validate(); err != nil {
		return nil, err
	}

	col := collector.NewCollector(l, exporterConfig)
	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(col),
	), nil
}

