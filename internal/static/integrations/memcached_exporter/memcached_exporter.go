// Package memcached_exporter embeds https://github.com/google/memcached_exporter
package memcached_exporter

import (
	"crypto/tls"
	"time"

	config_util "github.com/prometheus/common/config"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/memcached_exporter/pkg/exporter"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig is the default config for memcached_exporter.
var DefaultConfig = Config{
	MemcachedAddress: "localhost:11211",
	Timeout:          time.Second,
}

// Config controls the memcached_exporter integration.
type Config struct {
	// MemcachedAddress is the address of the memcached server (host:port).
	MemcachedAddress string `yaml:"memcached_address,omitempty"`

	// Timeout is the connection timeout for memcached.
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// TLSConfig is used to configure TLS for connection to memcached.
	TLSConfig *config_util.TLSConfig `yaml:"tls_config,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "memcached_exporter"
}

// InstanceKey returns the address:port of the memcached server.
func (c *Config) InstanceKey(_ string) (string, error) {
	return c.MemcachedAddress, nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("memcached"))
}

// New creates a new memcached_exporter integration. The integration scrapes metrics
// from a memcached server.
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	var tlsConfig *tls.Config
	var err error
	// NewTLSConfig uses Validate, which does not have a check if the config is nil,
	// so we need to check it
	if c.TLSConfig != nil {
		tlsConfig, err = config_util.NewTLSConfig(c.TLSConfig)
		if err != nil {
			level.Error(log).Log("msg", "invalid tls_config", "err", err)
			return nil, err
		}
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(
			// The memcached client does check if the tlsConfig is nil, so passing
			// nil here is fine.
			exporter.New(c.MemcachedAddress, c.Timeout, log, tlsConfig),
		),
	), nil
}
