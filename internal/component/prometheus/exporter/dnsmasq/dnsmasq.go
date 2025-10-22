package dnsmasq

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/dnsmasq_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.dnsmasq",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "dnsmasq"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default arguments for the prometheus.exporter.dnsmasq component.
var DefaultArguments = Arguments{
	Address:      "localhost:53",
	LeasesFile:   "/var/lib/misc/dnsmasq.leases",
	ExposeLeases: false,
}

// Arguments configures the prometheus.exporter.dnsmasq component.
type Arguments struct {
	// Address is the address of the dnsmasq server to connect to (host:port).
	Address string `alloy:"address,attr,optional"`

	// LeasesFile is the path to the dnsmasq leases file.
	LeasesFile string `alloy:"leases_file,attr,optional"`

	// ExposeLeases controls whether expose dnsmasq leases as metrics (high cardinality).
	ExposeLeases bool `alloy:"expose_leases,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a Arguments) Convert() *dnsmasq_exporter.Config {
	return &dnsmasq_exporter.Config{
		DnsmasqAddress: a.Address,
		LeasesPath:     a.LeasesFile,
		ExposeLeases:   a.ExposeLeases,
	}
}
