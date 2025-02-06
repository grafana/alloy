package mongodb

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mongodb",
		Stability: featuregate.StabilityGenerallyAvailable,
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
	URI                    alloytypes.Secret `alloy:"mongodb_uri,attr"`
	DirectConnect          bool              `alloy:"direct_connect,attr,optional"`
	DiscoveringMode        bool              `alloy:"discovering_mode,attr,optional"`
	TLSBasicAuthConfigPath string            `alloy:"tls_basic_auth_config_path,attr,optional"`
	CompatibleMode         bool              `alloy:"compatible_mode,attr,optional"`
}

func (a *Arguments) Convert() *mongodb_exporter.Config {
	return &mongodb_exporter.Config{
		URI:                    config_util.Secret(a.URI),
		DirectConnect:          a.DirectConnect,
		DiscoveringMode:        a.DiscoveringMode,
		TLSBasicAuthConfigPath: a.TLSBasicAuthConfigPath,
		CompatibleMode:         a.CompatibleMode,
	}
}
