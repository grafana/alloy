package mongodb_exporter //nolint:golint

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/go-kit/log"
	"github.com/percona/mongodb_exporter/exporter"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

var DefaultConfig = Config{
	DirectConnect: true,
}

// Config controls mongodb_exporter
type Config struct {
	// MongoDB connection URI. example:mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"
	URI                    config_util.Secret `yaml:"mongodb_uri"`
	DirectConnect          bool               `yaml:"direct_connect,omitempty"`
	DiscoveringMode        bool               `yaml:"discovering_mode,omitempty"`
	TLSBasicAuthConfigPath string             `yaml:"tls_basic_auth_config_path,omitempty"`
	CompatibleMode         bool               `yaml:"compatible_mode,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig
	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "mongodb_exporter"
}

// InstanceKey returns the address:port of the mongodb server being queried.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(string(c.URI))
	if err != nil {
		return "", fmt.Errorf("could not parse mongodb_uri: %w", errors.Unwrap(err))
	}
	return u.Host, nil
}

// NewIntegration creates a new mongodb_exporter
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("mongodb"))
}

// New creates a new mongodb_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	logrusLogger := integrations.NewLogger(logger)

	if c.TLSBasicAuthConfigPath != "" {
		if _, err := os.Stat(c.TLSBasicAuthConfigPath); err != nil {
			return nil, fmt.Errorf("tls config file path is invalid: %s. error: %w", c.TLSBasicAuthConfigPath, errors.Unwrap(err))
		}
	}

	exp := exporter.New(&exporter.Opts{
		URI:                    string(c.URI),
		Logger:                 logrusLogger,
		DisableDefaultRegistry: true,
		CompatibleMode:         c.CompatibleMode,
		CollectAll:             true,
		DirectConnect:          c.DirectConnect,
		DiscoveringMode:        c.DiscoveringMode,
		TLSConfigPath:          c.TLSBasicAuthConfigPath,
	})

	return integrations.NewHandlerIntegration(c.Name(), exp.Handler()), nil
}
