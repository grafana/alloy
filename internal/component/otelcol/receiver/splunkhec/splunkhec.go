package splunkhec

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"slices"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelconfig "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.splunkhec",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			f := splunkhecreceiver.NewFactory()
			return receiver.New(opts, f, args.(Arguments))
		},
	})
}

type SplittingStrategy string

const (
	SplittingStrategyNone SplittingStrategy = "none"
	SplittingStrategyLine SplittingStrategy = "line"
)

// MarshalText implements encoding.TextMarshaler
func (s SplittingStrategy) MarshalText() (text []byte, err error) {
	return []byte(s), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *SplittingStrategy) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "none":
		*s = SplittingStrategyNone
	case "line":
		*s = SplittingStrategyLine
	default:
		return fmt.Errorf("unknown splitting strategy: %s", str)
	}

	return nil
}

type Arguments struct {
	HTTPServer otelcol.HTTPServerArguments `alloy:",squash"`

	// RawPath for raw data collection. Optional.
	RawPath string `alloy:"raw_path,attr,optional"`

	// HealthPath for health API. Optional.
	HealthPath string `alloy:"health_path,attr,optional"`

	// Splitting strategy used, can be either "line" or "none". Optional.
	Splitting SplittingStrategy `alloy:"splitting,attr,optional"`

	// AccessTokenPassthrough if enabled preserves incoming access token as a attribute "com.splunk.hec.access_token".
	// `otelcol.exporter.splunkhec` will check for this attribute and if present forward it.
	AccessTokenPassthrough bool `alloy:"access_token_passthrough,attr,optional"`

	// HecToOtelAttrs creates a mapping from HEC metadata to attributes. Optional.
	HecToOtelAttrs HecToOtelAttrsArguments `alloy:"hec_metadata_to_otel_attrs,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelconfig.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *ConsumerArguments `alloy:"output,block"`
}

type ConsumerArguments struct {
	Metrics []otelcol.Consumer `alloy:"metrics,attr,optional"`
	Logs    []otelcol.Consumer `alloy:"logs,attr,optional"`
}

// HecToOtelAttrsArguments defines the mapping of Splunk HEC metadata to attributes.
type HecToOtelAttrsArguments struct {
	// Source indicates the mapping of the source field to a specific unified model attribute. Optional.
	Source string `alloy:"source,attr,optional"`
	// SourceType indicates the mapping of the sourcetype field to a specific unified model attribute. Optional.
	SourceType string `alloy:"sourcetype,attr,optional"`
	// Index indicates the mapping of the index field to a specific unified model attribute. Optional.
	Index string `alloy:"index,attr,optional"`
	// Host indicates the mapping of the host field to a specific unified model attribute. Optional.
	Host string `alloy:"host,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *HecToOtelAttrsArguments) SetToDefault() {
	*a = HecToOtelAttrsArguments{
		Source:     "com.splunk.source",
		SourceType: "com.splunk.sourcetype",
		Index:      "com.splunk.index",
		Host:       "host.name",
	}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		HTTPServer: otelcol.HTTPServerArguments{
			// Default value https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/4b29766cfde3a0eea46439707167084bfc48bad1/receiver/splunkhecreceiver/README.md#L28
			Endpoint:              "localhost:8088",
			CompressionAlgorithms: slices.Clone(otelcol.DefaultCompressionAlgorithms),
		},
		// Default value https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/9db5375e6a1092e3e93cd4743f257d30735be22c/internal/splunk/common.go#L32
		RawPath: "/services/collector/raw",
		// Default value https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/4b29766cfde3a0eea46439707167084bfc48bad1/receiver/splunkhecreceiver/config.go#L32
		HealthPath: "/services/collector/health",
		Splitting:  SplittingStrategyLine,
	}
	a.HecToOtelAttrs.SetToDefault()
	a.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (a Arguments) Convert() (otelcomponent.Config, error) {
	httpServerConfig, err := a.HTTPServer.ConvertToPtr()
	if err != nil {
		return nil, err
	}

	c := &splunkhecreceiver.Config{
		ServerConfig: *httpServerConfig,
		RawPath:      a.RawPath,
		HealthPath:   a.HealthPath,
		Splitting:    splunkhecreceiver.SplittingStrategy(a.Splitting),
	}

	c.AccessTokenPassthroughConfig.AccessTokenPassthrough = a.AccessTokenPassthrough
	c.HecToOtelAttrs.Source = a.HecToOtelAttrs.Source
	c.HecToOtelAttrs.SourceType = a.HecToOtelAttrs.SourceType
	c.HecToOtelAttrs.Index = a.HecToOtelAttrs.Index
	c.HecToOtelAttrs.Host = a.HecToOtelAttrs.Host

	return c, nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (a Arguments) DebugMetricsConfig() otelconfig.DebugMetricsArguments {
	return a.DebugMetrics
}

// Exporters implements receiver.Arguments.
func (a Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Extensions implements receiver.Arguments.
func (a Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	// FIXME(kalleep): Add support for ack extension.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.122.0/extension/ackextension
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if a.HTTPServer.Authentication != nil {
		ext, err := a.HTTPServer.Authentication.GetExtension(auth.Server)
		// Extension will not be registered if there was an error.
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
	}
	return m
}

// NextConsumers implements receiver.Arguments.
func (a Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return &otelcol.ConsumerArguments{
		Metrics: a.Output.Metrics,
		Logs:    a.Output.Logs,
	}
}
