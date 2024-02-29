package logs

import (
	"reflect"
	"time"

	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
)

// DefaultGlobalConfig holds default global settings to be used across all instances.
var DefaultGlobalConfig = GlobalConfig{
	ClientConfigs: []client.Config{},
	FileWatch: file.WatchConfig{
		MinPollFrequency: 250 * time.Millisecond,
		MaxPollFrequency: 250 * time.Millisecond,
	},
}

// GlobalConfig holds global settings that apply to all instances by default.
type GlobalConfig struct {
	FileWatch     file.WatchConfig `yaml:"file_watch_config,omitempty"`
	ClientConfigs []client.Config  `yaml:"clients,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *GlobalConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultGlobalConfig

	type plain GlobalConfig
	return unmarshal((*plain)(c))
}

func (c GlobalConfig) IsZero() bool {
	return reflect.DeepEqual(c, GlobalConfig{}) || reflect.DeepEqual(c, DefaultGlobalConfig)
}
