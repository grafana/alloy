// Package consul_exporter embeds https://github.com/prometheus/consul_exporter
package consul_exporter

import (
	"fmt"
	"net/url"
	"time"

	"github.com/go-kit/log"
	consul_api "github.com/hashicorp/consul/api"
	"github.com/prometheus/consul_exporter/pkg/exporter"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default settings for the consul_exporter integration.
var DefaultConfig = Config{
	Server:        "http://localhost:8500",
	Timeout:       500 * time.Millisecond,
	AllowStale:    true,
	KVFilter:      ".*",
	HealthSummary: true,
}

// Config controls the consul_exporter integration.
type Config struct {
	Server             string        `yaml:"server,omitempty"`
	CAFile             string        `yaml:"ca_file,omitempty"`
	CertFile           string        `yaml:"cert_file,omitempty"`
	KeyFile            string        `yaml:"key_file,omitempty"`
	ServerName         string        `yaml:"server_name,omitempty"`
	Timeout            time.Duration `yaml:"timeout,omitempty"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify,omitempty"`
	RequestLimit       int           `yaml:"concurrent_request_limit,omitempty"`
	AllowStale         bool          `yaml:"allow_stale,omitempty"`
	RequireConsistent  bool          `yaml:"require_consistent,omitempty"`

	KVPrefix      string `yaml:"kv_prefix,omitempty"`
	KVFilter      string `yaml:"kv_filter,omitempty"`
	HealthSummary bool   `yaml:"generate_health_summary,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "consul_exporter"
}

// InstanceKey returns the hostname:port of the Consul server.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(c.Server)
	if err != nil {
		return "", fmt.Errorf("could not parse url: %w", err)
	}
	return u.Host, nil
}

// NewIntegration converts the config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("consul"))
}

// New creates a new consul_exporter integration. The integration scrapes
// metrics from a consul process.
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	var (
		consulOpts = exporter.ConsulOpts{
			CAFile:       c.CAFile,
			CertFile:     c.CertFile,
			Insecure:     c.InsecureSkipVerify,
			KeyFile:      c.KeyFile,
			RequestLimit: c.RequestLimit,
			ServerName:   c.ServerName,
			Timeout:      c.Timeout,
			URI:          c.Server,
		}
		queryOptions = consul_api.QueryOptions{
			AllowStale:        c.AllowStale,
			RequireConsistent: c.RequireConsistent,
		}
	)

	e, err := exporter.New(consulOpts, queryOptions, c.KVPrefix, c.KVFilter, c.HealthSummary, log)
	if err != nil {
		return nil, err
	}

	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(e)), nil
}
