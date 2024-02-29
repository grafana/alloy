// Package basic provides an otelcol.auth.basic component.
package basic

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol/auth"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.basic",
		Stability: featuregate.StabilityStable,
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

	Username string            `river:"username,attr"`
	Password rivertypes.Secret `river:"password,attr"`
}

var _ auth.Arguments = Arguments{}

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
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}
