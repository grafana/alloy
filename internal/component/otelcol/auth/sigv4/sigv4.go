package sigv4

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.sigv4",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := sigv4authextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.auth.sigv4 component.
type Arguments struct {
	Region     string     `alloy:"region,attr,optional"`
	Service    string     `alloy:"service,attr,optional"`
	AssumeRole AssumeRole `alloy:"assume_role,block,optional"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ auth.Arguments = Arguments{}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// ConvertClient implements auth.Arguments.
func (args Arguments) ConvertClient() (otelcomponent.Config, error) {
	res := sigv4authextension.Config{
		Region:     args.Region,
		Service:    args.Service,
		AssumeRole: *args.AssumeRole.Convert(),
	}
	// sigv4authextension.Config has a private member called "credsProvider" which gets initialized when we call Validate().
	// If we don't call validate, the unit tests for this component will fail.
	if err := res.Validate(); err != nil {
		return nil, err
	}
	return &res, nil
}

// ConvertServer returns nil since the sigv4 extension does not support server authentication.
func (args Arguments) ConvertServer() (otelcomponent.Config, error) {
	return nil, nil
}

// Validate implements syntax.Validator.
func (args Arguments) Validate() error {
	_, err := args.ConvertClient()
	return err
}

// Extensions implements auth.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// AuthFeatures implements auth.Arguments.
func (args Arguments) AuthFeatures() auth.AuthFeature {
	return auth.ClientAuthSupported
}

// Exporters implements auth.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements auth.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// AssumeRole replicates sigv4authextension.Config.AssumeRole
type AssumeRole struct {
	ARN         string `alloy:"arn,attr,optional"`
	SessionName string `alloy:"session_name,attr,optional"`
	STSRegion   string `alloy:"sts_region,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *AssumeRole) Convert() *sigv4authextension.AssumeRole {
	if args == nil {
		return nil
	}

	return &sigv4authextension.AssumeRole{
		ARN:         args.ARN,
		SessionName: args.SessionName,
		STSRegion:   args.STSRegion,
	}
}
