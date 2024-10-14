// package splunkhec provides an otel.exporter splunkhec component

package splunkhec

import (
	"errors"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	splunkhec_config "github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelco.exporter.splunkhec",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := splunkhecexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

type Arguments struct {
	Client splunkhec_config.SplunkHecClientArguments `alloy:"client,block"`
	//Queue  otelcol.QueueArguments                    `alloy:"sending_queue,block,optional"`
	//Retry  otelcol.RetryArguments                    `alloy:"retry_on_failure,block,optional"`

	// Splunk specific configuration settings
	Splunk splunkhec_config.SplunkConf `alloy:"splunk,block"`
	// OnlyMetadata bool                        `alloy:"only_metadata,attr,optional"`
	// Hostname     string                      `alloy:"hostname,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	//args.Queue.SetToDefault()
	//args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.Splunk.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {

	return splunkhec_config.SplunkHecArguments{
		Splunk: args.Splunk,
		//QueueSettings:           *args.Queue.Convert(),
		SplunkHecClientArguments: args.Client,
	}, nil

}

func (args *Arguments) Validate() error {
	if args.Client.Endpoint == "" {
		return errors.New("missing hec endpoint")
	}
	if args.Splunk.Token == "" {
		return errors.New("missing token")
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
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}
