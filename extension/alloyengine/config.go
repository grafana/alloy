package alloyengine

import (
	"fmt"
	"os"
)

type Format string

const (
	FormatFile Format = "file"
)

// Config represents the configuration for the alloyflow extension.
type Config struct {
	Format Format            `mapstructure:"format"`
	Value  string            `mapstructure:"value"`
	Flags  map[string]string `mapstructure:"flags"`
}

func (cfg *Config) flagsAsSlice() []string {
	flags := []string{}
	for k, v := range cfg.Flags {
		flags = append(flags, fmt.Sprintf("--%s=%s", k, v))
	}
	return flags
}

func (f Format) validFormat() bool {
	switch f {
	case FormatFile:
		return true
	default:
		return false
	}
}

// Validate checks if the extension configuration is valid.
func (cfg *Config) Validate() error {
	if !cfg.Format.validFormat() {
		return fmt.Errorf("unsupported format: %q", cfg.Format)
	}

	if cfg.Value == "" {
		return fmt.Errorf("config value is required")
	}

	// Check if the config path exists and can be read
	_, err := os.Stat(cfg.Value)
	if err != nil {
		return fmt.Errorf("provided config path %s does not exist or is not readable: %w", cfg.Value, err)
	}

	return nil
}
