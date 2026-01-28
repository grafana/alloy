package integrations

import (
	"fmt"
	"time"

	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/metrics"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/prometheus/common/model"
	promConfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
)

var CurrentManagerConfig ManagerConfig = DefaultManagerConfig()

// DefaultManagerConfig holds the default settings for integrations.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		ScrapeIntegrations:        true,
		IntegrationRestartBackoff: 5 * time.Second,

		// Deprecated fields which keep their previous defaults:
		UseHostnameLabel:     true,
		ReplaceInstanceLabel: true,
	}
}

// ManagerConfig holds the configuration for all integrations.
type ManagerConfig struct {
	// When true, scrapes metrics from integrations.
	ScrapeIntegrations bool `yaml:"scrape_integrations,omitempty"`

	// The integration configs is merged with the manager config struct so we
	// don't want to export it here; we'll manually unmarshal it in UnmarshalYAML.
	Integrations Configs `yaml:"-"`

	// Extra labels to add for all integration samples
	Labels model.LabelSet `yaml:"labels,omitempty"`

	// Prometheus RW configs to use for all integrations.
	PrometheusRemoteWrite []*promConfig.RemoteWriteConfig `yaml:"prometheus_remote_write,omitempty"`

	IntegrationRestartBackoff time.Duration `yaml:"integration_restart_backoff,omitempty"`

	// ListenPort tells the integration Manager which port the Agent is
	// listening on for generating Prometheus instance configs.
	ListenPort int `yaml:"-"`

	// ListenHost tells the integration Manager which port the Agent is
	// listening on for generating Prometheus instance configs
	ListenHost string `yaml:"-"`

	TLSConfig config_util.TLSConfig `yaml:"http_tls_config,omitempty"`

	// This is set to true if the Server TLSConfig Cert and Key path are set
	ServerUsingTLS bool `yaml:"-"`

	// We use this config to check if we need to reload integrations or not
	// The Integrations Configs don't have prometheus defaults applied which
	// can cause us skip reload when scrape configs change
	PrometheusGlobalConfig promConfig.GlobalConfig `yaml:"-"`

	//
	// Deprecated and ignored fields.
	//

	ReplaceInstanceLabel bool `yaml:"replace_instance_label,omitempty"` // DEPRECATED, unused
	UseHostnameLabel     bool `yaml:"use_hostname_label,omitempty"`     // DEPRECATED, unused
}

// MarshalYAML implements yaml.Marshaler for ManagerConfig.
func (c ManagerConfig) MarshalYAML() (any, error) {
	return MarshalYAML(c)
}

// UnmarshalYAML implements yaml.Unmarshaler for ManagerConfig.
func (c *ManagerConfig) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultManagerConfig()
	return UnmarshalYAML(c, unmarshal)
}

// DefaultRelabelConfigs returns the set of relabel configs that should be
// prepended to all RelabelConfigs for an integration.
func (c *ManagerConfig) DefaultRelabelConfigs(instanceKey string) []*relabel.Config {
	return []*relabel.Config{{
		SourceLabels: model.LabelNames{model.AddressLabel},
		Action:       relabel.Replace,
		Separator:    ";",
		Regex:        relabel.MustNewRegexp("(.*)"),
		Replacement:  instanceKey,
		TargetLabel:  model.InstanceLabel,
	}}
}

// ApplyDefaults applies default settings to the ManagerConfig and validates
// that it can be used.
//
// If any integrations are enabled and are configured to be scraped, the
// Prometheus configuration must have a WAL directory configured.
func (c *ManagerConfig) ApplyDefaults(sflags *server.Flags, mcfg *metrics.Config) error {
	host, port, err := sflags.HTTP.ListenHostPort()
	if err != nil {
		return fmt.Errorf("reading HTTP host:port: %w", err)
	}

	c.ListenHost = host
	c.ListenPort = port
	c.ServerUsingTLS = sflags.HTTP.UseTLS

	if len(c.PrometheusRemoteWrite) == 0 {
		c.PrometheusRemoteWrite = mcfg.Global.RemoteWrite
	}
	c.PrometheusGlobalConfig = mcfg.Global.Prometheus

	for _, ic := range c.Integrations {
		if !ic.Common.Enabled {
			continue
		}

		scrapeIntegration := c.ScrapeIntegrations
		if common := ic.Common; common.ScrapeIntegration != nil {
			scrapeIntegration = *common.ScrapeIntegration
		}

		// WAL must be configured if an integration is going to be scraped.
		if scrapeIntegration && mcfg.WALDir == "" {
			return fmt.Errorf("no wal_directory configured")
		}
	}

	return nil
}
