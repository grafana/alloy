package squid_exporter

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	se "github.com/boynux/squid-exporter/collector"
	"github.com/go-kit/log"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"

	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

var (
	ErrNoAddress  = errors.New("no address was provided")
	ErrNoHostname = errors.New("no hostname in provided address")
	ErrNoPort     = errors.New("no port in provided address")
)

// Config is the configuration for the squid integration
type Config struct {
	Address  string             `yaml:"address"`
	Username string             `yaml:"username"`
	Password config_util.Secret `yaml:"password"`
	Host     string
	Port     int
}

func (c *Config) validate() error {
	if c.Address == "" {
		return ErrNoAddress
	}

	host, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		return err
	}

	if host == "" {
		return ErrNoHostname
	}
	c.Host = host

	if port == "" {
		return ErrNoPort
	}

	if sp, err := strconv.Atoi(port); err != nil {
		return err
	} else {
		c.Port = sp
		return nil
	}
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = Config{}

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the integration name this config is associated with.
func (c *Config) Name() string {
	return "squid"
}

// InstanceKey returns the addr of the squid instance.
func (c *Config) InstanceKey(_ string) (string, error) {
	return c.Address, nil
}

// NewIntegration returns the Squid Exporter Integration
func (c *Config) NewIntegration(log log.Logger) (integrations.Integration, error) {
	return New(c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("squid"))
}

// New creates a new squid integration. The integration scrapes metrics
// from an Squid exporter running with the https://github.com/boynux/squid-exporter
func New(c *Config) (integrations.Integration, error) {
	se.ExtractServiceTimes = true
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	seExporter := se.New(&se.CollectorConfig{
		Hostname: c.Host,
		Port:     c.Port,
		Login:    c.Username,
		Password: string(c.Password),
	})

	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(seExporter)), nil
}
