package logs

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/clients/pkg/promtail/limit"
	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
)

// Config controls the configuration of the Loki log scraper.
type Config struct {
	PositionsDirectory string            `yaml:"positions_directory,omitempty"`
	Global             GlobalConfig      `yaml:"global,omitempty"`
	Configs            []*InstanceConfig `yaml:"configs,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type config Config
	err := unmarshal((*config)(c))
	if err != nil {
		return err
	}

	return nil
}

// ApplyDefaults applies defaults to the Config and ensures that it is valid.
//
// Validations:
//
//  1. No two InstanceConfigs may have the same name.
//  2. No two InstanceConfigs may have the same positions path.
//  3. No InstanceConfig may have an empty name.
//  4. If InstanceConfig positions path is empty, shared PositionsDirectory
//     must not be empty.
//
// Defaults:
//
//  1. If a positions config is empty, it will be generated based on
//     the InstanceConfig name and Config.PositionsDirectory.
//  2. If an InstanceConfigs's ClientConfigs is empty, it will be generated based on
//     the Config.GlobalConfig.ClientConfigs.
func (c *Config) ApplyDefaults() error {
	var (
		names     = map[string]struct{}{}
		positions = map[string]string{} // positions file name -> config using it
	)

	for idx, ic := range c.Configs {
		if ic.Name == "" {
			return fmt.Errorf("Loki config index %d must have a name", idx)
		}
		if _, ok := names[ic.Name]; ok {
			return fmt.Errorf("found two Loki configs with name %s", ic.Name)
		}
		names[ic.Name] = struct{}{}

		if ic.PositionsConfig.PositionsFile == "" {
			if c.PositionsDirectory == "" {
				return fmt.Errorf("cannot generate Loki positions file path for %s because positions_directory is not configured", ic.Name)
			}
			ic.PositionsConfig.PositionsFile = filepath.Join(c.PositionsDirectory, ic.Name+".yml")
		}
		if orig, ok := positions[ic.PositionsConfig.PositionsFile]; ok {
			return fmt.Errorf("Loki configs %s and %s must have different positions file paths", orig, ic.Name)
		}
		positions[ic.PositionsConfig.PositionsFile] = ic.Name

		if len(ic.ClientConfigs) == 0 {
			ic.ClientConfigs = c.Global.ClientConfigs
		}
	}

	return nil
}

// InstanceConfig is an individual Promtail config.
type InstanceConfig struct {
	Name string `yaml:"name,omitempty"`

	ClientConfigs   []client.Config       `yaml:"clients,omitempty"`
	PositionsConfig positions.Config      `yaml:"positions,omitempty"`
	ScrapeConfig    []scrapeconfig.Config `yaml:"scrape_configs,omitempty"`
	TargetConfig    file.Config           `yaml:"target_config,omitempty"`
	LimitsConfig    limit.Config          `yaml:"limits_config,omitempty"`
}

func (c *InstanceConfig) Initialize() {
	// Defaults for Promtail are hidden behind flags. Register flags to a fake flagset
	// just to set the defaults in the configs.
	fs := flag.NewFlagSet("temp", flag.PanicOnError)
	c.PositionsConfig.RegisterFlags(fs)
	c.TargetConfig.RegisterFlags(fs)

	// Blank out the positions file since we set our own default for that.
	c.PositionsConfig.PositionsFile = ""
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *InstanceConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Initialize()
	type instanceConfig InstanceConfig
	return unmarshal((*instanceConfig)(c))
}
