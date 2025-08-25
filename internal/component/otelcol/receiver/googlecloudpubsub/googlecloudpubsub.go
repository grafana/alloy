package googlecloudpubsub

import (
	"fmt"
	"regexp"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.googlecloudpubsub",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return receiver.New(opts, googlecloudpubsubreceiver.NewFactory(), args.(Arguments))
		},
	})
}

type Arguments struct {
	ProjectID           string        `alloy:"project,attr,optional"`
	UserAgent           string        `alloy:"user_agent,attr,optional"`
	Endpoint            string        `alloy:"endpoint,attr,optional"`
	Insecure            bool          `alloy:"insecure,attr,optional"`
	Subscription        string        `alloy:"subscription,attr"`
	Encoding            string        `alloy:"encoding,attr,optional"`
	Compression         string        `alloy:"compression,attr,optional"`
	IgnoreEncodingError bool          `alloy:"ignore_encoding_error,attr,optional"`
	ClientID            string        `alloy:"client_id,attr,optional"`
	Timeout             time.Duration `alloy:"timeout,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var (
	subscriptionMatcher = regexp.MustCompile(`projects/[a-z][a-z0-9\-]*/subscriptions/`)

	_ receiver.Arguments = Arguments{}
	_ syntax.Defaulter   = &Arguments{}
	_ syntax.Validator   = &Arguments{}
)

func (args *Arguments) SetToDefault() {
	*args = Arguments{}

	args.UserAgent = "opentelemetry-collector-contrib {{version}}"
	args.DebugMetrics.SetToDefault()
	args.Timeout = time.Second * 12
}

func (args *Arguments) Validate() error {
	// duplicate the logic from (*googlecloudpubsubreceiver.Config).validate()
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42069

	if !subscriptionMatcher.MatchString(args.Subscription) {
		return fmt.Errorf("subscription '%s' is not a valid format, use 'projects/<project_id>/subscriptions/<name>'", args.Subscription)
	}

	switch args.Compression {
	case "":
	case "gzip":
	default:
		return fmt.Errorf("compression %v is not supported.  supported compression formats include [gzip]", args.Compression)
	}

	return nil
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	otelConfig := &googlecloudpubsubreceiver.Config{
		ProjectID:           args.ProjectID,
		UserAgent:           args.UserAgent,
		Endpoint:            args.Endpoint,
		Insecure:            args.Insecure,
		Subscription:        args.Subscription,
		Encoding:            args.Encoding,
		Compression:         args.Compression,
		IgnoreEncodingError: args.IgnoreEncodingError,
		ClientID:            args.ClientID,
		TimeoutSettings: exporterhelper.TimeoutConfig{
			Timeout: args.Timeout,
		},
	}

	return otelConfig, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
