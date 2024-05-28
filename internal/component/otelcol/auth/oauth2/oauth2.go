package oauth2

import (
	"net/url"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.oauth2",
		Stability: featuregate.StabilityGenerallyAvailable,
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
	ClientID         string                     `alloy:"client_id,attr,optional"`
	ClientIDFile     string                     `alloy:"client_id_file,attr,optional"`
	ClientSecret     alloytypes.Secret          `alloy:"client_secret,attr,optional"`
	ClientSecretFile string                     `alloy:"client_secret_file,attr,optional"`
	TokenURL         string                     `alloy:"token_url,attr"`
	EndpointParams   url.Values                 `alloy:"endpoint_params,attr,optional"`
	Scopes           []string                   `alloy:"scopes,attr,optional"`
	TLSSetting       otelcol.TLSClientArguments `alloy:"tls,block,optional"`
	Timeout          time.Duration              `alloy:"timeout,attr,optional"`
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

// DebugMetricsConfig implements auth.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
