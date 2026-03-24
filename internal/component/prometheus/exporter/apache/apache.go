package apache

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/apache_http"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.apache",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "apache"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the apache exporter
var DefaultArguments = Arguments{
	ApacheAddr:         "http://localhost/server-status?auto",
	ApacheHostOverride: "",
	ApacheInsecure:     false,
}

// Arguments controls the apache exporter.
type Arguments struct {
	ApacheAddr         string `alloy:"scrape_uri,attr,optional"`
	ApacheHostOverride string `alloy:"host_override,attr,optional"`
	ApacheInsecure     bool   `alloy:"insecure,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *apache_http.Config {
	return &apache_http.Config{
		ApacheAddr:         a.ApacheAddr,
		ApacheHostOverride: a.ApacheHostOverride,
		ApacheInsecure:     a.ApacheInsecure,
	}
}
