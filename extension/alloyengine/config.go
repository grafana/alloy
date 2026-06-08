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
	// ModulePath is value to be resolved for "module_path" alloy config keyword.
	//
	// Has no effect if [File] is set.
	ModulePath string `mapstructure:"module_path"`

	// File is a path to Alloy config file or a directory containing config files.
	//
	// Note: either [File] or [Content] can be set.
	File string `mapstructure:"file"`

	// Content is config contents.
	//
	// Note: either [File] or [Content] can be set.
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
