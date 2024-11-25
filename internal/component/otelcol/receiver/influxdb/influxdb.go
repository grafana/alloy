// influxdb.go
package influxdb

import (
    "fmt"

    "github.com/grafana/alloy/internal/component"
    "github.com/grafana/alloy/internal/component/otelcol"
    otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
    "github.com/grafana/alloy/internal/component/otelcol/receiver"
    "github.com/grafana/alloy/internal/featuregate"
    influxdbreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver"
    otelcomponent "go.opentelemetry.io/collector/component"
    otelextension "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/pipeline"
)

func init() {
    component.Register(component.Registration{
        Name:      "otelcol.receiver.influxdb",
        Stability: featuregate.StabilityGenerallyAvailable,
        Args:      Arguments{},

        Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
            fact := influxdbreceiver.NewFactory()
            return receiver.New(opts, fact, args.(Arguments))
        },
    })
}

// Arguments configures the otelcol.receiver.influxdb component.
type Arguments struct {
    HTTPServer    otelcol.HTTPServerArguments         `alloy:",squash"` // Use custom struct for HTTP
    DebugMetrics  otelcolCfg.DebugMetricsArguments   `alloy:"debug_metrics,block,optional"`
    Output        *otelcol.ConsumerArguments         `alloy:"output,block"`
}
type HTTPServerArguments struct {
    Endpoint              string   `alloy:"endpoint,attr"`
    // CompressionAlgorithms []string `alloy:"compression_algorithms,attr,optional"`
}
// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
    args.HTTPServer = otelcol.HTTPServerArguments{
        Endpoint:              "localhost:8086",
        // CompressionAlgorithms: []string{"gzip", "zstd"},
    }
    args.DebugMetrics.SetToDefault()
}

// Validate ensures that the Arguments configuration is valid.
func (args *Arguments) Validate() error {
    if args.HTTPServer.Endpoint == "" {
        return fmt.Errorf("HTTP server endpoint cannot be empty")
    }
    return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
    return &influxdbreceiver.Config{
        ServerConfig: *args.HTTPServer.Convert(),
    }, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
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
