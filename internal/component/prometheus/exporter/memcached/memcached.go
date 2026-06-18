package memcached

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/memcached_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.memcached",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "memcached"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default arguments for the prometheus.exporter.memcached component.
var DefaultArguments = Arguments{
	Address: "localhost:11211",
	Timeout: time.Second,
}

// Arguments configures the prometheus.exporter.memcached component.
type Arguments struct {
	// Address is the address of the memcached server to connect to (host:port).
	Address string `alloy:"address,attr,optional"`

	// Timeout is the timeout for the memcached exporter to use when connecting to the
	// memcached server.
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	// TLSConfig is used to configure TLS for connection to memcached.
	TLSConfig *config.TLSConfig `alloy:"tls_config,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	if a.TLSConfig == nil {
		return nil
	}
	return a.TLSConfig.Validate()
}

func (a Arguments) Convert() *memcached_exporter.Config {
	return &memcached_exporter.Config{
		MemcachedAddress: a.Address,
		Timeout:          a.Timeout,
		TLSConfig:        a.TLSConfig.Convert(),
	}
}
