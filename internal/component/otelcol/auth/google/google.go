package google

import (
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	collectorgoogleauth "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.google",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := collectorgoogleauth.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.google component.
type Arguments struct {
	// Project is the project telemetry is sent to if the gcp.project.id
	// resource attribute is not set. If unspecified, this is determined using
	// application default credentials.
	Project string `alloy:"project,attr,optional"`

	// QuotaProject specifies a project for quota and billing purposes. The
	// caller must have serviceusage.services.use permission on the project.
	//
	// For more information please read:
	// https://cloud.google.com/apis/docs/system-parameters
	QuotaProject string `alloy:"quota_project,attr,optional"`

	// TokenType specifies which type of token will be generated.
	// default: access_token
	TokenType string `alloy:"token_type,attr,optional"`

	// Audience specifies the audience claim used for generating ID token.
	Audience string `alloy:"audience,attr,optional"`

	// Scope specifies optional requested permissions.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.3
	Scopes []string `alloy:"scopes,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ auth.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	upstreamDefault := googleclientauthextension.CreateDefaultConfig().(*googleclientauthextension.Config)
	// TODO: The upstream default config currently reuses a pointer to scopes.
	// Fix this in the upstream repo and then change this to
	// upstreamDefault.Scopes.
	args.Scopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/logging.write",
		"https://www.googleapis.com/auth/monitoring.write",
		"https://www.googleapis.com/auth/trace.append",
	}
	args.TokenType = upstreamDefault.TokenType
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator
func (args Arguments) Validate() error {
	converted, _ := args.ConvertClient()
	return converted.(*collectorgoogleauth.Config).Validate()
}

// ConvertClient implements auth.Arguments.
func (args Arguments) ConvertClient() (otelcomponent.Config, error) {
	return &collectorgoogleauth.Config{
		Config: googleclientauthextension.Config{
			Project:      args.Project,
			QuotaProject: args.QuotaProject,
			TokenType:    args.TokenType,
			Audience:     args.Audience,
			Scopes:       args.Scopes,
		},
	}, nil
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
