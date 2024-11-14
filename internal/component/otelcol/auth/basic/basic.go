// Package basic provides an otelcol.auth.basic component.
package basic

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.basic",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := basicauthextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.basic component.
type Arguments struct {
	// TODO(rfratto): should we support htpasswd?

	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ auth.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// Convert implements auth.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &basicauthextension.Config{
		ClientAuth: &basicauthextension.ClientAuthSettings{
			Username: args.Username,
			Password: configopaque.String(args.Password),
		},
	}, nil
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements auth.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
