package eventhandler

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/prometheus/prometheus/model/labels"
)

// DefaultConfig sets defaults for Config
var DefaultConfig = Config{
	SendTimeout:    60,
	CachePath:      "./.eventcache/eventhandler.cache",
	LogsInstance:   "default",
	InformerResync: 120,
	FlushInterval:  10,
	LogFormat:      "logfmt",
}

// Config configures the eventhandler integration
type Config struct {
	// Eventhandler hands watched events off to promtail using a promtail
	// client channel. This parameter configures how long to wait (in seconds) on the channel
	// before abandoning and moving on.
	SendTimeout int `yaml:"send_timeout,omitempty"`
	// Configures the path to a kubeconfig file. If not set, will fall back to using
	// an in-cluster config. If this fails, will fall back to checking the user's home
	// directory for a kubeconfig.
	KubeconfigPath string `yaml:"kubeconfig_path,omitempty"`
	// Path to a cache file that will store the last timestamp for a shipped event and events
	// shipped for that timestamp. Used to prevent double-shipping on integration restart.
	CachePath string `yaml:"cache_path,omitempty"`
	// Name of logs subsystem instance to hand log entries off to.
	LogsInstance string `yaml:"logs_instance,omitempty"`
	// K8s informer resync interval (seconds). You should use defaults here unless you are
	// familiar with K8s informers.
	InformerResync int `yaml:"informer_resync,omitempty"`
	// The integration will flush the last event shipped out to disk every flush_interval seconds.
	FlushInterval int `yaml:"flush_interval,omitempty"`
	// If you would like to limit events to a given namespace, use this parameter.
	Namespace string `yaml:"namespace,omitempty"`
	// Extra labels to append to log lines
	ExtraLabels labels.Labels `yaml:"extra_labels,omitempty"`
	// For changing the log format to json, use this parameter.
	LogFormat   string  `yaml:"log_format,omitempty"`
	InstanceKey *string `yaml:"instance,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents
func (c *Config) Name() string { return "eventhandler" }

// ApplyDefaults applies runtime-specific defaults to c
func (c *Config) ApplyDefaults(globals integrations.Globals) error {
	return nil
}

// Identifier uniquely identifies this instance of Config
func (c *Config) Identifier(globals integrations.Globals) (string, error) {
	if c.InstanceKey != nil {
		return *c.InstanceKey, nil
	}
	return c.Name(), nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger, globals integrations.Globals) (integrations.Integration, error) {
	// NOTE(rfratto): the eventhandler integration is never run, and all the
	// logic has been moved to the loki.source.kubernetes_events component.
	//
	// This function is never called, but still exists for config conversion.
	return stubIntegration{}, nil
}

func init() {
	integrations.Register(&Config{}, integrations.TypeSingleton)
}

type stubIntegration struct{}

func (stubIntegration) RunIntegration(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
