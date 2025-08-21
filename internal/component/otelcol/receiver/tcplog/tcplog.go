// Package tcplog provides an otelcol.receiver.tcplog component.
package tcplog

import (
	"fmt"
	"net"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/textutils"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/hashicorp/go-multierror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	stanzainputtcp "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

// init registers the tcplog component in the Alloy ecosystem.
func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.tcplog",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := tcplogreceiver.NewFactory()
			return receiver.New(opts, factory, args.(Arguments))
		},
	})
}

// Values taken from tcp input Build function
const tcpDefaultMaxLogSize = 1024 * 1024
const minMaxLogSize = helper.ByteSize(64 * 1024)

// Arguments configures the otelcol.receiver.tcplog component.
type Arguments struct {
	ListenAddress   string                      `alloy:"listen_address,attr"`
	MaxLogSize      units.Base2Bytes            `alloy:"max_log_size,attr,optional"`
	TLS             *otelcol.TLSServerArguments `alloy:"tls,block,optional"`
	AddAttributes   bool                        `alloy:"add_attributes,attr,optional"`
	OneLogPerPacket bool                        `alloy:"one_log_per_packet,attr,optional"`
	Encoding        string                      `alloy:"encoding,attr,optional"`
	MultilineConfig *MultilineConfig            `alloy:"multiline,block,optional"`

	ConsumerRetry otelcol.ConsumerRetryArguments `alloy:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

type MultilineConfig struct {
	LineStartPattern string `alloy:"line_start_pattern,attr,optional"`
	LineEndPattern   string `alloy:"line_end_pattern,attr,optional"`
	OmitPattern      bool   `alloy:"omit_pattern,attr,optional"`
}

func (c *MultilineConfig) Convert() *split.Config {
	if c == nil {
		return nil
	}

	return &split.Config{
		LineStartPattern: c.LineStartPattern,
		LineEndPattern:   c.LineEndPattern,
		OmitPattern:      c.OmitPattern,
	}
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter, providing default values.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		MaxLogSize: tcpDefaultMaxLogSize,
		Output:     &otelcol.ConsumerArguments{},
	}
	args.DebugMetrics.SetToDefault()
	args.ConsumerRetry.SetToDefault()
}

// Convert implements receiver.Arguments, converting these Arguments
// into an OpenTelemetry Collector config object for the tcplogreceiver.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	c := stanzainputtcp.NewConfig()
	tls := args.TLS.Convert()
	c.BaseConfig = stanzainputtcp.BaseConfig{
		MaxLogSize:      helper.ByteSize(args.MaxLogSize),
		ListenAddress:   args.ListenAddress,
		TLS:             tls.Get(),
		AddAttributes:   args.AddAttributes,
		OneLogPerPacket: args.OneLogPerPacket,
		Encoding:        args.Encoding,
	}
	split := args.MultilineConfig.Convert()
	if split != nil {
		c.SplitConfig = *split
	}

	def := tcplogreceiver.ReceiverType{}.CreateDefaultConfig()
	cfg := def.(*tcplogreceiver.TCPLogConfig)
	cfg.InputConfig = *c

	// consumerretry package is stanza internal so we can't just Convert
	cfg.RetryOnFailure.Enabled = args.ConsumerRetry.Enabled
	cfg.RetryOnFailure.InitialInterval = args.ConsumerRetry.InitialInterval
	cfg.RetryOnFailure.MaxInterval = args.ConsumerRetry.MaxInterval
	cfg.RetryOnFailure.MaxElapsedTime = args.ConsumerRetry.MaxElapsedTime

	return cfg, nil
}

// Extensions implements receiver.Arguments, returning any needed extensions.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements receiver.Arguments, returning exporters by signal type.
// This wrapper doesn't add any exporters internally; it defers to Output.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments, returning the next consumer pipeline.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// Validate implements syntax.Validator for basic argument checks.
func (args *Arguments) Validate() error {
	var errs error

	if err := validateListenAddress(args.ListenAddress, "listen_address"); err != nil {
		errs = multierror.Append(errs, err)
	}

	_, err := textutils.LookupEncoding(args.Encoding)
	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("invalid encoding: %w", err))
	}

	if int64(args.MaxLogSize) < int64(minMaxLogSize) {
		errs = multierror.Append(errs, fmt.Errorf("invalid value %d for parameter 'max_log_size', must be equal to or greater than %d bytes", args.MaxLogSize, minMaxLogSize))
	}

	return errs
}

func validateListenAddress(url string, urlName string) error {
	if url == "" {
		return fmt.Errorf("%s cannot be empty", urlName)
	}

	if _, _, err := net.SplitHostPort(url); err != nil {
		return fmt.Errorf("invalid %s: %w", urlName, err)
	}
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
