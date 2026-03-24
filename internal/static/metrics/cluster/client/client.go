package client

import (
	"flag"
	"reflect"

	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/dskit/grpcclient"
)

var (
	// DefaultConfig provides default Config values.
	DefaultConfig = *util.DefaultConfigFromFlags(&Config{}).(*Config)
)

// Config controls how scraping service clients are created.
type Config struct {
	GRPCClientConfig grpcclient.Config `yaml:"grpc_client_config,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c Config) IsZero() bool {
	return reflect.DeepEqual(c, Config{}) || reflect.DeepEqual(c, DefaultConfig)
}

// RegisterFlags registers flags to the provided flag set.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("prometheus.", f)
	c.RegisterFlagsWithPrefix("metrics.", f)
}

// RegisterFlagsWithPrefix registers flags to the provided flag set with the
// specified prefix.
func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	c.GRPCClientConfig.RegisterFlagsWithPrefix(prefix+"service-client", f)
}
