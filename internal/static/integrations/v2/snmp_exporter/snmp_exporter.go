// Package snmp_exporter embeds https://github.com/prometheus/snmp_exporter
package snmp_exporter_v2

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

// DefaultConfig holds the default settings for the snmp_exporter integration.
var DefaultConfig = Config{
	WalkParams:              make(map[string]snmp_config.WalkParams),
	SnmpConfigFile:          "",
	SnmpConfigMergeStrategy: "replace",
	SnmpConcurrency:         1,
}

// Config configures the SNMP integration.
type Config struct {
	WalkParams              map[string]snmp_config.WalkParams `yaml:"walk_params,omitempty"`
	SnmpConfigFile          string                            `yaml:"config_file,omitempty"`
	SnmpConfigMergeStrategy string                            `yaml:"config_merge_strategy,omitempty"`
	SnmpConcurrency         int                               `yaml:"concurrency,omitempty"`
	SnmpTargets             []snmp_exporter.SNMPTarget        `yaml:"snmp_targets"`
	SnmpConfig              snmp_config.Config                `yaml:"snmp_config,omitempty"`
	Common                  common.MetricsConfig              `yaml:",inline"`

	globals integrations_v2.Globals
}

// ApplyDefaults applies the integration's default configuration.
func (c *Config) ApplyDefaults(globals integrations_v2.Globals) error {
	c.Common.ApplyDefaults(globals.SubsystemOpts.Metrics.Autoscrape)
	return nil
}

// Identifier returns a string that identifies the integration.
func (c *Config) Identifier(globals integrations_v2.Globals) (string, error) {
	if c.Common.InstanceKey != nil {
		return *c.Common.InstanceKey, nil
	}
	return c.Name(), nil
}

// NewIntegration creates a new SNMP integration.
func (c *Config) NewIntegration(log log.Logger, globals integrations_v2.Globals) (integrations_v2.Integration, error) {
	snmpCfg, err := snmp_exporter.LoadSNMPConfig(c.SnmpConfigFile, &c.SnmpConfig, c.SnmpConfigMergeStrategy)
	if err != nil {
		return nil, err
	}
	c.globals = globals
	sh := &snmpHandler{
		cfg:     c,
		snmpCfg: snmpCfg,
		log:     log,
	}
	return sh, nil
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	// This should technically be accomplished by assigning DefaultConfig, right?
	// But in the reload case the existing values in this map are not purged and
	// an unmarshal error is thrown stating that they key already exists in the map.
	c.WalkParams = make(map[string]snmp_config.WalkParams)

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "snmp"
}

func init() {
	integrations_v2.Register(&Config{}, integrations_v2.TypeSingleton)
}
