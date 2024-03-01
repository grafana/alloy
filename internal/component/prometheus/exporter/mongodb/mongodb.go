package mongodb

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mongodb",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mongodb"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	URI                    rivertypes.Secret `river:"mongodb_uri,attr"`
	DirectConnect          bool              `river:"direct_connect,attr,optional"`
	DiscoveringMode        bool              `river:"discovering_mode,attr,optional"`
	TLSBasicAuthConfigPath string            `river:"tls_basic_auth_config_path,attr,optional"`
}

func (a *Arguments) Convert() *mongodb_exporter.Config {
	return &mongodb_exporter.Config{
		URI:                    config_util.Secret(a.URI),
		DirectConnect:          a.DirectConnect,
		DiscoveringMode:        a.DiscoveringMode,
		TLSBasicAuthConfigPath: a.TLSBasicAuthConfigPath,
	}
}
