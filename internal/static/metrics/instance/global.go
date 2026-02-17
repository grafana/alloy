package instance

import (
	"reflect"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
)

// DefaultGlobalConfig holds default global settings to be used across all instances.
var DefaultGlobalConfig = globalConfig()

func globalConfig() GlobalConfig {
	cfg := GlobalConfig{Prometheus: config.DefaultGlobalConfig}
	// We use `DefaultScrapeProtocols` to keep the native histograms disabled by default.
	// See https://github.com/prometheus/prometheus/pull/12738/files#diff-17f1012e0c2fbd9bcd8dff3c23b18ff4b6676eef3beca6f8a3e72e6a36633334R64-R68
	cfg.Prometheus.ScrapeProtocols = config.DefaultScrapeProtocols
	cfg.Prometheus.MetricNameValidationScheme = model.UTF8Validation
	return cfg
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
func (c *GlobalConfig) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultGlobalConfig

	type plain GlobalConfig
	return unmarshal((*plain)(c))
}

func (c GlobalConfig) IsZero() bool {
	return reflect.DeepEqual(c, GlobalConfig{}) || reflect.DeepEqual(c, DefaultGlobalConfig)
}
