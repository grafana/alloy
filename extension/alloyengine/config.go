package alloyengine

import (
	"fmt"
)

type Config struct {
	AlloyConfig AlloyConfig       `mapstructure:"config"`
	Flags       map[string]string `mapstructure:"flags"`
}

// AlloyConfig represents the incoming format of the Alloy configuration.
type AlloyConfig struct {
	Content string `mapstructure:"content"`
}

func (cfg *Config) flagsAsSlice() []string {
	flags := []string{}
	for k, v := range cfg.Flags {
		flags = append(flags, fmt.Sprintf("--%s=%s", k, v))
	}
	return flags
}

func (cfg *Config) Validate() error {
	if cfg.AlloyConfig.Content == "" {
		return fmt.Errorf("config.content is required")
	}

	return nil
}
