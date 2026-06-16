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
	// Path is a path to Alloy config file or a directory containing config files.
	//
	// Note: either [Path] or [Inline] can be set.
	Path string `mapstructure:"path"`

	// Inline is the inline Alloy configuration.
	//
	// Note: either [Path] or [Inline] can be set.
	Inline InlineAlloyConfig `mapstructure:"inline"`
}

type InlineAlloyConfig struct {
	// ModulePath is value to be resolved for "module_path" alloy config keyword.
	ModulePath string `mapstructure:"module_path"`

	// Content is the inline Alloy config content.
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
	hasPath := cfg.AlloyConfig.Path != ""
	hasContent := cfg.AlloyConfig.Inline.Content != ""

	if !hasPath && !hasContent {
		return fmt.Errorf("either config.path or config.inline.content must be set")
	}
	if hasPath && hasContent {
		return fmt.Errorf("exactly one of config.path or config.inline.content must be set")
	}
	if cfg.AlloyConfig.Inline.ModulePath != "" && hasPath {
		return fmt.Errorf("config.inline.module_path has no effect when config.path is set")
	}

	return nil
}
