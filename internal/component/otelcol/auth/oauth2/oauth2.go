package oauth2

import (
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/auth"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.oauth2",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := oauth2clientauthextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.oauth2 component.
type Arguments struct {
	ClientID         string                     `river:"client_id,attr,optional"`
	ClientIDFile     string                     `river:"client_id_file,attr,optional"`
	ClientSecret     rivertypes.Secret          `river:"client_secret,attr,optional"`
	ClientSecretFile string                     `river:"client_secret_file,attr,optional"`
	TokenURL         string                     `river:"token_url,attr"`
	EndpointParams   url.Values                 `river:"endpoint_params,attr,optional"`
	Scopes           []string                   `river:"scopes,attr,optional"`
	TLSSetting       otelcol.TLSClientArguments `river:"tls,block,optional"`
	Timeout          time.Duration              `river:"timeout,attr,optional"`
}

var _ auth.Arguments = Arguments{}

// Convert implements auth.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &oauth2clientauthextension.Config{
		ClientID:         args.ClientID,
		ClientIDFile:     args.ClientIDFile,
		ClientSecret:     configopaque.String(args.ClientSecret),
		ClientSecretFile: args.ClientSecretFile,
		TokenURL:         args.TokenURL,
		EndpointParams:   args.EndpointParams,
		Scopes:           args.Scopes,
		TLSSetting:       *args.TLSSetting.Convert(),
		Timeout:          args.Timeout,
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
