package squid

import (
	"net"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/squid_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.squid",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "squid"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// Arguments controls the squid exporter.
type Arguments struct {
	SquidAddr     string            `alloy:"address,attr"`
	SquidUser     string            `alloy:"username,attr,optional"`
	SquidPassword alloytypes.Secret `alloy:"password,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.SquidAddr == "" {
		return squid_exporter.ErrNoAddress
	}

	host, port, err := net.SplitHostPort(a.SquidAddr)
	if err != nil {
		return err
	}

	if host == "" {
		return squid_exporter.ErrNoHostname
	}

	if port == "" {
		return squid_exporter.ErrNoPort
	}

	return nil
}

func (a *Arguments) Convert() *squid_exporter.Config {
	return &squid_exporter.Config{
		Address:  a.SquidAddr,
		Username: a.SquidUser,
		Password: config.Secret(a.SquidPassword),
	}
}
