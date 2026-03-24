package alloyengine

import (
	"fmt"
	"os"
)

type Config struct {
	AlloyConfig AlloyConfig       `mapstructure:"config"`
	Flags       map[string]string `mapstructure:"flags"`
}

// This type represents the incoming format of the Alloy configuration
// This is a one-of type, and it is expected that only one of the fields will be set (ie, we cannot define multiple config sources of different types)
type AlloyConfig struct {
	File string `mapstructure:"file"`
}

func (cfg *Config) flagsAsSlice() []string {
	flags := []string{}
	for k, v := range cfg.Flags {
		flags = append(flags, fmt.Sprintf("--%s=%s", k, v))
	}
	return flags
}

func (cfg *Config) Validate() error {
	if cfg.AlloyConfig.File == "" {
		return fmt.Errorf("config.file is required")
	}

	_, err := os.Stat(cfg.AlloyConfig.File)
	if err != nil {
		return fmt.Errorf("provided config path %s does not exist or is not readable: %w", cfg.AlloyConfig.File, err)
	}

	return nil
}
