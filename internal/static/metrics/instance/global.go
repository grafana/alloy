package instance

import (
	"reflect"
	"time"

	"github.com/prometheus/prometheus/config"
)

// DefaultGlobalConfig holds default global settings to be used across all instances.
var DefaultGlobalConfig = GlobalConfig{
	Prometheus: config.DefaultGlobalConfig,
}

// GlobalConfig holds global settings that apply to all instances by default.
type GlobalConfig struct {
	Prometheus  config.GlobalConfig         `yaml:",inline"`
	RemoteWrite []*config.RemoteWriteConfig `yaml:"remote_write,omitempty"`

	ExtraMetrics      bool          `yaml:"-"`
	DisableKeepAlives bool          `yaml:"-"`
	IdleConnTimeout   time.Duration `yaml:"-"`
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
