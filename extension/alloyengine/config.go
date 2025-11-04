package alloyengine

import (
	"fmt"
	"os"
)

// Config represents the configuration for the alloyflow extension.
type Config struct {
	ConfigPath string            `mapstructure:"config_path"`
	Flags      map[string]string `mapstructure:"flags"`
}

func (cfg *Config) flagsAsSlice() []string {
	flags := []string{}
	for k, v := range cfg.Flags {
		flags = append(flags, fmt.Sprintf("--%s=%s", k, v))
	}
	return flags
}

// Validate checks if the extension configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.ConfigPath == "" {
		return fmt.Errorf("config_path is required")
	}
	// Check if the config path exists and can be read
	_, err := os.Stat(cfg.ConfigPath)
	if err != nil {
		return fmt.Errorf("config_path %s does not exist or is not readable: %w", cfg.ConfigPath, err)
	}

	return nil
}
