// Package basic provides an otelcol.auth.basic component.
package basic

import (
	"errors"
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pipeline"
)

var (
	errNoCredentialSource = errors.New("no credential source provided")
	errNoPasswordProvided = errors.New("no password provided")
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

type HtpasswdConfig struct {
	File   string `alloy:"file,attr,optional"`
	Inline string `alloy:"inline,attr,optional"`
}

func (c HtpasswdConfig) convert() *basicauthextension.HtpasswdSettings {
	return &basicauthextension.HtpasswdSettings{
		File:   c.File,
		Inline: c.Inline,
	}
}

type ClientAuthConfig struct {
	Username string `alloy:"username,attr"`
	Password string `alloy:"password,attr"`
}

func (c ClientAuthConfig) convert() *basicauthextension.ClientAuthSettings {
	if c.Username == "" && c.Password == "" {
		return nil
	}
	return &basicauthextension.ClientAuthSettings{
		Username: c.Username,
		Password: configopaque.String(c.Password),
	}
}

// Arguments configures the otelcol.auth.basic component.
type Arguments struct {
	Username string            `alloy:"username,attr,optional"` // Deprecated: Use ClientAuth instead
	Password alloytypes.Secret `alloy:"password,attr,optional"` // Deprecated: Use ClientAuth instead

	ClientAuth *ClientAuthConfig `alloy:"client_auth,block,optional"`

	Htpasswd *HtpasswdConfig `alloy:"htpasswd,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ auth.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator
func (args Arguments) Validate() error {
	// check if no argument was provided
	if args.Username == "" && args.Password == "" && args.Htpasswd == nil && args.ClientAuth == nil {
		return errNoCredentialSource
	}
	// the downstream basicauthextension package supports having both inline
	// and htpasswd files, so we should not error out in case both are
	// provided

	// check if password was not provided when username is provided
	if args.Username != "" && args.Password == "" {
		return errNoPasswordProvided
	}

	return nil
}

// ConvertClient implements auth.Arguments.
func (args Arguments) ConvertClient() (otelcomponent.Config, error) {
	c := &basicauthextension.Config{}
	// If the client config is specified, ignore the deprecated
	// username and password attributes.
	if args.ClientAuth != nil {
		c.ClientAuth = args.ClientAuth.convert()
		return c, nil
	}

	c.ClientAuth = &basicauthextension.ClientAuthSettings{
		Username: args.Username,
		Password: configopaque.String(args.Password),
	}
	return c, nil
}

// ConvertServer implements auth.Arguments.
func (args Arguments) ConvertServer() (otelcomponent.Config, error) {
	c := &basicauthextension.Config{
		Htpasswd: &basicauthextension.HtpasswdSettings{},
	}
	if args.Htpasswd != nil {
		c.Htpasswd = args.Htpasswd.convert()
	}
	// Keeping this to avoid breaking existing use cases. Remove this for v2
	if args.Username != "" && args.Password != "" {
		c.Htpasswd.Inline += fmt.Sprintf("\n%s:%s", args.Username, args.Password)
	}

	return c, nil
}

// AuthFeatures implements auth.Arguments.
func (args Arguments) AuthFeatures() auth.AuthFeature {
	return auth.ClientAndServerAuthSupported
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
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
