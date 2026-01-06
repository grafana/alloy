package faro

import (
	"maps"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.faro",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := faroexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeLogs|exporter.TypeTraces))
		},
	})
}

type Arguments struct {
	Client HTTPClientArguments    `alloy:"client,block"`
	Queue  otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry  otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	httpClientArgs := *(*otelcol.HTTPClientArguments)(&args.Client)
	convertedClientArgs, err := httpClientArgs.Convert()
	if err != nil {
		return nil, err
	}
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	return &faroexporter.Config{
		ClientConfig: *convertedClientArgs,
		QueueConfig:  q,
		RetryConfig:  *args.Retry.Convert(),
	}, nil
}

func (args *Arguments) Validate() error {
	if err := args.Queue.Validate(); err != nil {
		return err
	}
	if err := args.Retry.Validate(); err != nil {
		return err
	}
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}
	faroCfg := otelCfg.(*faroexporter.Config)
	return faroCfg.Validate()
}
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	ext := (*otelcol.HTTPClientArguments)(&args.Client).Extensions()
	maps.Copy(ext, args.Queue.Extensions())
	return ext
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// HTTPClientArguments is used to configure otelcol.exporter.faro with
// component-specific defaults.
type HTTPClientArguments otelcol.HTTPClientArguments

// SetToDefault implements syntax.Defaulter.
func (args *HTTPClientArguments) SetToDefault() {
	*args = HTTPClientArguments{
		Timeout:           30 * time.Second,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		Headers:           map[string]string{},
		Compression:       otelcol.CompressionTypeGzip,
		WriteBufferSize:   512 * 1024,
		ForceAttemptHTTP2: true,
	}
}
