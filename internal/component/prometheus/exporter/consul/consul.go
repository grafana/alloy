package consul

import (
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/consul_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.consul",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "consul"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
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
	Server             string        `river:"server,attr,optional"`
	CAFile             string        `river:"ca_file,attr,optional"`
	CertFile           string        `river:"cert_file,attr,optional"`
	KeyFile            string        `river:"key_file,attr,optional"`
	ServerName         string        `river:"server_name,attr,optional"`
	Timeout            time.Duration `river:"timeout,attr,optional"`
	InsecureSkipVerify bool          `river:"insecure_skip_verify,attr,optional"`
	RequestLimit       int           `river:"concurrent_request_limit,attr,optional"`
	AllowStale         bool          `river:"allow_stale,attr,optional"`
	RequireConsistent  bool          `river:"require_consistent,attr,optional"`

	KVPrefix      string `river:"kv_prefix,attr,optional"`
	KVFilter      string `river:"kv_filter,attr,optional"`
	HealthSummary bool   `river:"generate_health_summary,attr,optional"`
}

// SetToDefault implements river.Defaulter.
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
