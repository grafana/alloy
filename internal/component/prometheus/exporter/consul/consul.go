package consul

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/consul_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.consul",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "consul"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the consul_exporter exporter.
var DefaultArguments = Arguments{
	Server:        "http://localhost:8500",
	Timeout:       500 * time.Millisecond,
	AllowStale:    true,
	KVFilter:      ".*",
	HealthSummary: true,
}

// Arguments controls the consul_exporter exporter.
type Arguments struct {
	Server             string        `alloy:"server,attr,optional"`
	CAFile             string        `alloy:"ca_file,attr,optional"`
	CertFile           string        `alloy:"cert_file,attr,optional"`
	KeyFile            string        `alloy:"key_file,attr,optional"`
	ServerName         string        `alloy:"server_name,attr,optional"`
	Timeout            time.Duration `alloy:"timeout,attr,optional"`
	InsecureSkipVerify bool          `alloy:"insecure_skip_verify,attr,optional"`
	RequestLimit       int           `alloy:"concurrent_request_limit,attr,optional"`
	AllowStale         bool          `alloy:"allow_stale,attr,optional"`
	RequireConsistent  bool          `alloy:"require_consistent,attr,optional"`

	KVPrefix      string `alloy:"kv_prefix,attr,optional"`
	KVFilter      string `alloy:"kv_filter,attr,optional"`
	HealthSummary bool   `alloy:"generate_health_summary,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *consul_exporter.Config {
	return &consul_exporter.Config{
		Server:             a.Server,
		CAFile:             a.CAFile,
		CertFile:           a.CertFile,
		KeyFile:            a.KeyFile,
		ServerName:         a.ServerName,
		Timeout:            a.Timeout,
		InsecureSkipVerify: a.InsecureSkipVerify,
		RequestLimit:       a.RequestLimit,
		AllowStale:         a.AllowStale,
		RequireConsistent:  a.RequireConsistent,
		KVPrefix:           a.KVPrefix,
		KVFilter:           a.KVFilter,
		HealthSummary:      a.HealthSummary,
	}
}
