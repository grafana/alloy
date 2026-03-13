package google

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	collectorgoogleauth "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/pipeline"
	"google.golang.org/grpc/credentials"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.google",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			// Wrap the factory to intercept the created OpenTelemetry extension.
			fact := collectorgoogleauth.NewFactory()
			wrappedFact := &factoryWrapper{Factory: fact}

			// Intercept OnStateChange to capture the auth.Handler. This is required
			// to access the underlying extension so we can manually invoke Start().
			var handler *auth.Handler
			originalOnStateChange := opts.OnStateChange
			opts.OnStateChange = func(e component.Exports) {
				if exports, ok := e.(auth.Exports); ok {
					handler = exports.Handler
				}
				originalOnStateChange(e)
			}

			authComp, err := auth.New(opts, wrappedFact, args.(Arguments))
			if err != nil {
				return nil, err
			}

			// Return our wrapper component to ensure synchronous initialization.
			ga := &googleAuth{
				Auth: authComp,
				getHandler: func() *auth.Handler {
					return handler
				},
			}
			ga.syncStart()

			return ga, nil
		},
	})
}

// factoryWrapper proxies the OpenTelemetry Factory to return our extWrapper,
// allowing us to intercept the created extension and control its lifecycle.
type factoryWrapper struct {
	otelextension.Factory
}

func (f *factoryWrapper) Create(ctx context.Context, set otelextension.Settings, cfg otelcomponent.Config) (otelextension.Extension, error) {
	ext, err := f.Factory.Create(ctx, set, cfg)
	if err != nil {
		return nil, err
	}
	return &extWrapper{Extension: ext}, nil
}

// extWrapper wraps the OpenTelemetry Extension to ensure Start() is only called once.
// This prevents duplicate executions when invoked synchronously during Update() and
// asynchronously by the Alloy scheduler. It also proxies HTTP/gRPC authenticator interfaces.
type extWrapper struct {
	otelextension.Extension
	startOnce sync.Once
	err       error
}

func (e *extWrapper) Start(ctx context.Context, host otelcomponent.Host) error {
	e.startOnce.Do(func() {
		e.err = e.Extension.Start(ctx, host)
	})
	return e.err
}

func (e *extWrapper) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if client, ok := e.Extension.(extensionauth.HTTPClient); ok {
		return client.RoundTripper(base)
	}
	return nil, errors.New("not a HTTP client authenticator")
}

func (e *extWrapper) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	if client, ok := e.Extension.(extensionauth.GRPCClient); ok {
		return client.PerRPCCredentials()
	}
	return nil, errors.New("not a gRPC client authenticator")
}

// googleAuth wraps the Alloy auth.Auth component to allow synchronously invoking
// Start() during Update(). This guarantees the auth extension is fully initialized
// before dependent exporters attempt to use its credentials.
type googleAuth struct {
	*auth.Auth
	getHandler func() *auth.Handler
}

func (g *googleAuth) syncStart() {
	h := g.getHandler()
	if h != nil {
		ext, err := h.GetExtension(auth.Client)
		if err == nil && ext != nil && ext.Extension != nil {
			_ = ext.Extension.Start(context.Background(), nil)
		}
	}
}

func (g *googleAuth) Update(args component.Arguments) error {
	err := g.Auth.Update(args)
	if err != nil {
		return err
	}
	g.syncStart()
	return nil
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
