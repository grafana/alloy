package squid

import (
	"net"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/squid_exporter"
	"github.com/grafana/river/rivertypes"
	"github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.squid",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "squid"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// Arguments controls the squid exporter.
type Arguments struct {
	SquidAddr     string            `river:"address,attr"`
	SquidUser     string            `river:"username,attr,optional"`
	SquidPassword rivertypes.Secret `river:"password,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{}
}

// Validate implements river.Validator.
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
