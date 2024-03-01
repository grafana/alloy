// Package bearer provides an otelcol.auth.bearer component.
package bearer

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol/auth"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.bearer",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := bearertokenauthextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.bearer component.
type Arguments struct {
	// Do not include the "filename" attribute - users should use local.file instead.
	Scheme string            `river:"scheme,attr,optional"`
	Token  rivertypes.Secret `river:"token,attr"`
}

var _ auth.Arguments = Arguments{}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	Scheme: "Bearer",
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Convert implements auth.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &bearertokenauthextension.Config{
		Scheme:      args.Scheme,
		BearerToken: configopaque.String(args.Token),
	}, nil
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}
