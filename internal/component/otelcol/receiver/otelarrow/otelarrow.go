package otelarrow

import (
	"maps"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/pipeline"
)

// init registers the otelcol.receiver.otelarrow component.
func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.otelarrow",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelarrowreceiver.NewFactory()
			return receiver.New(opts, factory, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.otelarrow component.
type Arguments struct {
	GRPC      *GRPCServerArguments      `alloy:"grpc,block,optional"`
	Arrow     *ArrowConfigArguments     `alloy:"arrow,block,optional"`
	Admission *AdmissionConfigArguments `alloy:"admission,block,optional"`

	// DebugMetrics configures internal metrics, optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures the next consumers for the pipeline.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// Ensure Arguments implements receiver.Arguments.
var _ receiver.Arguments = Arguments{}

// SetToDefault populates default values on the receiver arguments.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		GRPC:         new(GRPCServerArguments),
		Arrow:        new(ArrowConfigArguments),
		Admission:    new(AdmissionConfigArguments),
		DebugMetrics: otelcolCfg.DebugMetricsArguments{},
		Output:       new(otelcol.ConsumerArguments),
	}
	args.GRPC.SetToDefault()
	args.Arrow.SetToDefault()
	args.Admission.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert transforms Arguments into a config that can be decoded into otelarrowreceiver.Config.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	// Create the config from the factory to ensure default values
	factory := otelarrowreceiver.NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*otelarrowreceiver.Config)

	// Convert GRPC server settings.
	if args.GRPC != nil {
		c, err := args.GRPC.Convert()
		if err != nil {
			return nil, err
		}
		defaultCfg.Protocols.GRPC = *c
	} else {
		// Use default GRPC config if not provided.
		def := GRPCServerArguments{}
		def.SetToDefault()
		c, err := def.Convert()
		if err != nil {
			return nil, err
		}
		defaultCfg.Protocols.GRPC = *c
	}

	// Configure Arrow settings
	if args.Arrow != nil {
		defaultCfg.Protocols.Arrow.MemoryLimitMiB = args.Arrow.MemoryLimitMiB

		// Note: The Zstd field in ArrowConfig requires access to internal types that
		// are not directly accessible. We currently don't set these values even if they're
		// specified in the arguments. This is a known limitation.
	}

	// Configure Admission settings
	if args.Admission != nil {
		defaultCfg.Admission.RequestLimitMiB = args.Admission.RequestLimitMiB
		defaultCfg.Admission.WaitingLimitMiB = args.Admission.WaitingLimitMiB
	}

	return defaultCfg, nil
}

// Extensions collects any server-side auth extensions, etc.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	result := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.GRPC != nil {
		grpcExts := (*otelcol.GRPCServerArguments)(args.GRPC).Extensions()
		maps.Copy(result, grpcExts)
	}
	return result
}

// Exporters returns nil as this component does not use additional exporters.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers returns the configuration specifying where to send the received data.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig returns the debug metrics settings.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// GRPCServerArguments aliases otelcol.GRPCServerArguments for component-specific defaults.
type GRPCServerArguments otelcol.GRPCServerArguments

// SetToDefault sets default gRPC server settings.
func (g *GRPCServerArguments) SetToDefault() {
	*g = GRPCServerArguments{
		Endpoint:       "0.0.0.0:4317",
		Transport:      "tcp",
		ReadBufferSize: 512 * units.Kibibyte,
	}
}

// Convert aliases the conversion method from otelcol.GRPCServerArguments.
func (g *GRPCServerArguments) Convert() (*configgrpc.ServerConfig, error) {
	return (*otelcol.GRPCServerArguments)(g).Convert()
}

// ArrowConfigArguments defines Arrow-specific configuration options.
type ArrowConfigArguments struct {
	MemoryLimitMiB uint64               `alloy:"memory_limit_mib,attr,optional"`
	Zstd           *ZstdConfigArguments `alloy:"zstd,block,optional"`
}

// SetToDefault sets default values for ArrowConfigArguments.
func (a *ArrowConfigArguments) SetToDefault() {
	*a = ArrowConfigArguments{
		MemoryLimitMiB: 0, // Default value, adjust as needed
		Zstd:           new(ZstdConfigArguments),
	}
	a.Zstd.SetToDefault()
}

// ZstdConfigArguments configures Zstd compression settings.
// Note: Due to limitations in accessing internal types, these settings may not be applied.
type ZstdConfigArguments struct {
	MemoryLimitMiB uint64 `alloy:"memory_limit_mib,attr,optional"`
	WindowSizeMib  uint32 `alloy:"window_size_mib,attr,optional"`
	Concurrency    uint   `alloy:"concurrency,attr,optional"`
}

// SetToDefault sets default values for ZstdConfigArguments.
func (z *ZstdConfigArguments) SetToDefault() {
	*z = ZstdConfigArguments{
		MemoryLimitMiB: 0, // Default value, adjust as needed
		WindowSizeMib:  0,
		Concurrency:    1,
	}
}

// AdmissionConfigArguments defines Admission-specific configuration options.
type AdmissionConfigArguments struct {
	RequestLimitMiB uint64 `alloy:"request_limit_mib,attr,optional"`
	WaitingLimitMiB uint64 `alloy:"waiting_limit_mib,attr,optional"`
}

// SetToDefault sets default values for AdmissionConfigArguments.
func (a *AdmissionConfigArguments) SetToDefault() {
	*a = AdmissionConfigArguments{
		RequestLimitMiB: 0, // Default value, adjust as needed
		WaitingLimitMiB: 0,
	}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// Add validation logic here if needed
	return nil
}
