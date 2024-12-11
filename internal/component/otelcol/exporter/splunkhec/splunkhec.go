// package splunkhec provides an otel.exporter splunkhec component
// Maintainers for the Grafana Alloy wrapper:
// - @adlotsof
// - @PatMis16
package splunkhec

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	splunkhec_config "github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.splunkhec",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := splunkhecexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

type Arguments struct {
	Client splunkhec_config.SplunkHecClientArguments `alloy:"client,block"`
	Queue  otelcol.QueueArguments                    `alloy:"sending_queue,block,optional"`
	Retry  otelcol.RetryArguments                    `alloy:"retry_on_failure,block,optional"`

	// Splunk specific configuration settings
	Splunk splunkhec_config.SplunkConf `alloy:"splunk,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.Splunk.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {

	return (&splunkhec_config.SplunkHecArguments{
		Splunk:                   args.Splunk,
		QueueSettings:            *args.Queue.Convert(),
		RetrySettings:            *args.Retry.Convert(),
		SplunkHecClientArguments: args.Client,
	}).Convert(), nil
}

func (args *Arguments) Validate() error {
	if err := args.Client.Validate(); err != nil {
		return err
	}
	if err := args.Splunk.Validate(); err != nil {
		return err
	}
	if err := args.Queue.Validate(); err != nil {
		return err
	}

	return nil
}
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}
