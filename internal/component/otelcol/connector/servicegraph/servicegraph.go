package servicegraph

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.servicegraph",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := servicegraphconnector.NewFactory()
			return connector.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.servicegraph component.
type Arguments struct {
	// LatencyHistogramBuckets is the list of durations representing latency histogram buckets.
	LatencyHistogramBuckets []time.Duration `alloy:"latency_histogram_buckets,attr,optional"`

	// Dimensions defines the list of additional dimensions on top of the provided:
	// - client
	// - server
	// - failed
	// - connection_type
	// The dimensions will be fetched from the span's attributes. Examples of some conventionally used attributes:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go.
	Dimensions []string `alloy:"dimensions,attr,optional"`

	// Store contains the config for the in-memory store used to find requests between services by pairing spans.
	Store StoreConfig `alloy:"store,block,optional"`
	// CacheLoop defines how often to clean the cache of stale series.
	CacheLoop time.Duration `alloy:"cache_loop,attr,optional"`
	// StoreExpirationLoop defines how often to expire old entries from the store.
	StoreExpirationLoop time.Duration `alloy:"store_expiration_loop,attr,optional"`
	// VirtualNodePeerAttributes the list of attributes need to match, the higher the front, the higher the priority.
	//TODO: Add VirtualNodePeerAttributes when it's no longer controlled by
	// the "processor.servicegraph.virtualNode" feature gate.
	// VirtualNodePeerAttributes []string `alloy:"virtual_node_peer_attributes,attr,optional"`

	// MetricsFlushInterval is the interval at which metrics are flushed to the exporter.
	// If set to 0, metrics are flushed on every received batch of traces.
	MetricsFlushInterval time.Duration `alloy:"metrics_flush_interval,attr,optional"`

	// DatabaseNameAttribute is the attribute name used to identify the database name from span attributes.
	// The default value is db.name
	// Deprecated: [v0.124.0] Use database_name_attributes instead.
	DatabaseNameAttribute string `alloy:"database_name_attribute,attr,optional"`

	// DatabaseNameAttributes is the attribute name list of attributes need to match used to identify the database name from span attributes, the higher the front, the higher the priority.
	// The default value is {"db.name"}.
	DatabaseNameAttributes []string `alloy:"database_name_attributes,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type StoreConfig struct {
	// MaxItems is the maximum number of items to keep in the store.
	MaxItems int `alloy:"max_items,attr,optional"`
	// TTL is the time to live for items in the store.
	TTL time.Duration `alloy:"ttl,attr,optional"`
}

func (sc *StoreConfig) SetToDefault() {
	*sc = StoreConfig{
		MaxItems: 1000,
		TTL:      2 * time.Second,
	}
}

var (
	_ syntax.Validator = (*Arguments)(nil)
	_ syntax.Defaulter = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		LatencyHistogramBuckets: []time.Duration{
			2 * time.Millisecond,
			4 * time.Millisecond,
			6 * time.Millisecond,
			8 * time.Millisecond,
			10 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			200 * time.Millisecond,
			400 * time.Millisecond,
			800 * time.Millisecond,
			1 * time.Second,
			1400 * time.Millisecond,
			2 * time.Second,
			5 * time.Second,
			10 * time.Second,
			15 * time.Second,
		},
		Dimensions:             []string{},
		CacheLoop:              1 * time.Minute,
		StoreExpirationLoop:    2 * time.Second,
		DatabaseNameAttributes: []string{"db.name"},
		MetricsFlushInterval:   60 * time.Second,
		//TODO: Add VirtualNodePeerAttributes when it's no longer controlled by
		// the "processor.servicegraph.virtualNode" feature gate.
		// VirtualNodePeerAttributes: []string{
		// 	semconv.AttributeDBName,
		// 	semconv.AttributeNetSockPeerAddr,
		// 	semconv.AttributeNetPeerName,
		// 	semconv.AttributeRPCService,
		// 	semconv.AttributeNetSockPeerName,
		// 	semconv.AttributeNetPeerName,
		// 	semconv.AttributeHTTPURL,
		// 	semconv.AttributeHTTPTarget,
		// },
	}
	args.Store.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.CacheLoop <= 0 {
		return fmt.Errorf("cache_loop must be greater than 0")
	}

	if args.StoreExpirationLoop <= 0 {
		return fmt.Errorf("store_expiration_loop must be greater than 0")
	}

	if args.Store.MaxItems <= 0 {
		return fmt.Errorf("store.max_items must be greater than 0")
	}

	if args.Store.TTL <= 0 {
		return fmt.Errorf("store.ttl must be greater than 0")
	}

	return nil
}

// Convert implements connector.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &servicegraphconnector.Config{
		// Never set a metric exporter.
		// The consumer of metrics will be set via Otel's Connector API.
		//
		// MetricsExporter:         "",
		LatencyHistogramBuckets: args.LatencyHistogramBuckets,
		Dimensions:              args.Dimensions,
		Store: servicegraphconnector.StoreConfig{
			MaxItems: args.Store.MaxItems,
			TTL:      args.Store.TTL,
		},
		CacheLoop:              args.CacheLoop,
		StoreExpirationLoop:    args.StoreExpirationLoop,
		MetricsFlushInterval:   &args.MetricsFlushInterval,
		DatabaseNameAttribute:  args.DatabaseNameAttribute,
		DatabaseNameAttributes: args.DatabaseNameAttributes,
		//TODO: Add VirtualNodePeerAttributes when it's no longer controlled by
		// the "processor.servicegraph.virtualNode" feature gate.
		// VirtualNodePeerAttributes: args.VirtualNodePeerAttributes,
	}, nil
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType() int implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorTracesToMetrics
}

// DebugMetricsConfig implements connector.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
