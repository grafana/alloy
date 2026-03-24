package vmware_exporter_v2

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/grafana/vmware_exporter/vsphere"
	config_util "github.com/prometheus/common/config"
)

func init() {
	integrations.Register(&Config{}, integrations.TypeMultiplex)
}

// DefaultConfig holds non-zero default options for hte Config when it is
// unmarshaled from YAML.
var DefaultConfig = Config{
	ChunkSize:               256,
	CollectConcurrency:      8,
	ObjectDiscoveryInterval: 0,
	EnableExporterMetrics:   true,
}

// Config configures the vmware_exporter integration.
type Config struct {
	ChunkSize               int                  `yaml:"request_chunk_size,omitempty"`
	CollectConcurrency      int                  `yaml:"collect_concurrency,omitempty"`
	VSphereURL              string               `yaml:"vsphere_url,omitempty"`
	VSphereUser             string               `yaml:"vsphere_user,omitempty"`
	VSpherePass             config_util.Secret   `yaml:"vsphere_password,omitempty"`
	ObjectDiscoveryInterval time.Duration        `yaml:"discovery_interval,omitempty"`
	EnableExporterMetrics   bool                 `yaml:"enable_exporter_metrics,omitempty"`
	Common                  common.MetricsConfig `yaml:",inline"`
}

var _ integrations.Config = (*Config)(nil)

// UnmarshalYAML implements the Unmarshaler interface.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig
	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of this integration.
func (c *Config) Name() string {
	return "vsphere"
}

// ApplyDefaults applies the integration's default configuration.
func (c *Config) ApplyDefaults(g integrations.Globals) error {
	c.Common.ApplyDefaults(g.SubsystemOpts.Metrics.Autoscrape)
	return nil
}

// Identifier returns a string that identifies the instance of the integration.
func (c *Config) Identifier(g integrations.Globals) (string, error) {
	if c.Common.InstanceKey != nil {
		return *c.Common.InstanceKey, nil
	}

	u, err := url.Parse(c.VSphereURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", u.Hostname(), u.Port()), nil
}

// NewIntegration constructs a new instance of this integration.
func (c *Config) NewIntegration(log log.Logger, g integrations.Globals) (integrations.Integration, error) {
	vsphereURL, err := url.Parse(c.VSphereURL)
	if err != nil {
		return nil, err
	}
	vsphereURL.User = url.UserPassword(c.VSphereUser, string(c.VSpherePass))

	exporterConfig := vsphere.Config{
		ChunkSize:               c.ChunkSize,
		CollectConcurrency:      c.CollectConcurrency,
		VSphereURL:              vsphereURL,
		ObjectDiscoveryInterval: c.ObjectDiscoveryInterval,
		EnableExporterMetrics:   c.EnableExporterMetrics,
	}
	exporter, err := vsphere.NewExporter(slog.New(logging.NewSlogGoKitHandler(log)), &exporterConfig)
	if err != nil {
		return nil, err
	}

	return metricsutils.NewMetricsHandlerIntegration(
		log, c, c.Common, g, exporter,
	)
}
