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
	errNoCredentialSource = errors.New("no credential source provided") //nolint:gofmt
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
	settings := &basicauthextension.HtpasswdSettings{}
	if c.File != "" {
		settings.File = c.File
	}
	if c.Inline != "" {
		settings.Inline = c.Inline
	}
	return settings
}

// Arguments configures the otelcol.auth.basic component.
type Arguments struct {
	Username string            `alloy:"username,attr,optional"`
	Password alloytypes.Secret `alloy:"password,attr,optional"`

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
	if args.Username == "" && args.Password == "" && args.Htpasswd == nil {
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
	return &basicauthextension.Config{
		ClientAuth: &basicauthextension.ClientAuthSettings{
			Username: args.Username,
			Password: configopaque.String(args.Password),
		},
	}, nil
}

// ConvertServer implements auth.Arguments.
func (args Arguments) ConvertServer() (otelcomponent.Config, error) {
	c := &basicauthextension.Config{
		Htpasswd: &basicauthextension.HtpasswdSettings{},
	}
	if args.Htpasswd != nil {
		c.Htpasswd = args.Htpasswd.convert()
	}
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
