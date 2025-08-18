package googlecloudpubsub

import (
	"fmt"
	"regexp"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
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
	ProjectID           string `alloy:"project,attr,optional"`
	UserAgent           string `alloy:"user_agent,attr,optional"`
	Endpoint            string `alloy:"endpoint,attr,optional"`
	Insecure            bool   `alloy:"insecure,attr,optional"`
	Subscription        string `alloy:"subscription,attr"`
	Encoding            string `alloy:"encoding,attr,optional"`
	Compression         string `alloy:"compression,attr,optional"`
	IgnoreEncodingError bool   `alloy:"ignore_encoding_error,attr,optional"`
	ClientID            string `alloy:"client_id,attr,optional"`

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
}

func (args *Arguments) Validate() error {
	otelConfig, err := args.Convert()
	if err != nil {
		return err
	}

	// duplicate the logic from (*googlecloudpubsubreceiver.Config).validate()
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42069

	googleCloudPubSubCfg, _ := otelConfig.(*googlecloudpubsubreceiver.Config)

	if !subscriptionMatcher.MatchString(googleCloudPubSubCfg.Subscription) {
		return fmt.Errorf("subscription '%s' is not a valid format, use 'projects/<project_id>/subscriptions/<name>'", googleCloudPubSubCfg.Subscription)
	}

	switch googleCloudPubSubCfg.Compression {
	case "":
	case "gzip":
	default:
		return fmt.Errorf("compression %v is not supported.  supported compression formats include [gzip]", googleCloudPubSubCfg.Compression)
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
	}

	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/googlecloudpubsubreceiver/config.go#L24
	otelConfig.TimeoutSettings.Timeout = 12 * time.Second

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
