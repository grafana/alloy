// Package syslog provides an otelcol.exporter.syslog component.
package syslog

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.syslog",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := syslogexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeLogs))
		},
	})
}

// Arguments configures the otelcol.exporter.syslog component.
type Arguments struct {
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	TLS otelcol.TLSClientArguments `alloy:"tls,block,optional"`

	Endpoint string              `alloy:"endpoint,attr"`
	Port     int                 `alloy:"port,attr,optional"`     // default: 514
	Network  string              `alloy:"network,attr,optional"`  // default: "tcp", also supported "udp"
	Protocol config.SysLogFormat `alloy:"protocol,attr,optional"` // default: "rfc5424", also supported "rfc3164"

	// Whether or not to enable RFC 6587 Octet Counting.
	EnableOctetCounting bool `alloy:"enable_octet_counting,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Timeout:  otelcol.DefaultTimeout,
		Port:     514,
		Network:  string(confignet.TransportTypeTCP),
		Protocol: config.SyslogFormatRFC5424,
	}

	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	return &syslogexporter.Config{
		TimeoutSettings: otelpexporterhelper.TimeoutConfig{
			Timeout: args.Timeout,
		},
		QueueSettings:       q,
		BackOffConfig:       *args.Retry.Convert(),
		Endpoint:            args.Endpoint,
		Port:                args.Port,
		Network:             args.Network,
		Protocol:            string(args.Protocol),
		TLS:                 *args.TLS.Convert(),
		EnableOctetCounting: args.EnableOctetCounting,
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
