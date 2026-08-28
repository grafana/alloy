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
	"go.opentelemetry.io/collector/pipeline"
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
	ClientID                 string                     `alloy:"client_id,attr,optional"`
	ClientIDFile             string                     `alloy:"client_id_file,attr,optional"`
	ClientSecret             alloytypes.Secret          `alloy:"client_secret,attr,optional"`
	ClientSecretFile         string                     `alloy:"client_secret_file,attr,optional"`
	ClientCertificateKeyID   string                     `alloy:"client_certificate_key_id,attr,optional"`
	ClientCertificateKey     alloytypes.Secret          `alloy:"client_certificate_key,attr,optional"`
	ClientCertificateKeyFile string                     `alloy:"client_certificate_key_file,attr,optional"`
	GrantType                string                     `alloy:"grant_type,attr,optional"`
	SignatureAlgorithm       string                     `alloy:"signature_algorithm,attr,optional"`
	Iss                      string                     `alloy:"iss,attr,optional"`
	Audience                 string                     `alloy:"audience,attr,optional"`
	Claims                   map[string]any             `alloy:"claims,attr,optional"`
	TokenURL                 string                     `alloy:"token_url,attr"`
	EndpointParams           url.Values                 `alloy:"endpoint_params,attr,optional"`
	Scopes                   []string                   `alloy:"scopes,attr,optional"`
	TLS                      otelcol.TLSClientArguments `alloy:"tls,block,optional"`
	Timeout                  time.Duration              `alloy:"timeout,attr,optional"`
	ExpiryBuffer             time.Duration              `alloy:"expiry_buffer,attr,optional"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ auth.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	cfg := oauth2clientauthextension.NewFactory().CreateDefaultConfig().(*oauth2clientauthextension.Config)
	args.GrantType = "client_credentials"
	args.ExpiryBuffer = cfg.ExpiryBuffer

	args.DebugMetrics.SetToDefault()
}

// ConvertClient implements auth.Arguments.
func (args Arguments) ConvertClient() (otelcomponent.Config, error) {
	cfg := oauth2clientauthextension.NewFactory().CreateDefaultConfig().(*oauth2clientauthextension.Config)
	cfg.ClientID = args.ClientID
	cfg.ClientIDFile = args.ClientIDFile
	cfg.ClientSecret = configopaque.String(args.ClientSecret)
	cfg.ClientSecretFile = args.ClientSecretFile
	cfg.ClientCertificateKeyID = args.ClientCertificateKeyID
	cfg.ClientCertificateKey = configopaque.String(args.ClientCertificateKey)
	cfg.ClientCertificateKeyFile = args.ClientCertificateKeyFile

	cfg.Iss = args.Iss
	cfg.Audience = args.Audience
	cfg.Claims = args.Claims
	cfg.TokenURL = args.TokenURL
	cfg.EndpointParams = args.EndpointParams
	cfg.Scopes = args.Scopes
	cfg.TLS = *args.TLS.Convert()
	cfg.Timeout = args.Timeout
	cfg.ExpiryBuffer = args.ExpiryBuffer

	if args.GrantType != "" {
		cfg.GrantType = args.GrantType
	}

	if args.SignatureAlgorithm != "" {
		cfg.SignatureAlgorithm = args.SignatureAlgorithm
	}

	return cfg, nil
}

// ConvertServer returns nil since the ouath2 client extension does not support server auth.
func (args Arguments) ConvertServer() (otelcomponent.Config, error) {
	return nil, nil
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// AuthFeatures implements auth.Arguments.
func (args Arguments) AuthFeatures() auth.AuthFeature {
	return auth.ClientAuthSupported
}

// DebugMetricsConfig implements auth.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
