// Package syslog provides an otelcol.receiver.syslog component.
package syslog

import (
	"fmt"
	"net"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/textutils"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/hashicorp/go-multierror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	stanzainputsyslog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"
	stanzainputtcp "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	stanzainputudp "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	stanzaparsersyslog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.syslog",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := syslogreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.syslog component.
type Arguments struct {
	Protocol                     config.SysLogFormat `alloy:"protocol,attr,optional"`
	Location                     string              `alloy:"location,attr,optional"`
	EnableOctetCounting          bool                `alloy:"enable_octet_counting,attr,optional"`
	MaxOctets                    int                 `alloy:"max_octets,attr,optional"`
	AllowSkipPriHeader           bool                `alloy:"allow_skip_pri_header,attr,optional"`
	NonTransparentFramingTrailer *FramingTrailer     `alloy:"non_transparent_framing_trailer,attr,optional"`

	ConsumerRetry otelcol.ConsumerRetryArguments `alloy:"retry_on_failure,block,optional"`
	TCP           *TCP                           `alloy:"tcp,block,optional"`
	UDP           *UDP                           `alloy:"udp,block,optional"`

	OnError string `alloy:"on_error,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

type FramingTrailer string

var NULTrailer FramingTrailer = "NUL"
var LFTrailer FramingTrailer = "LF"

// MarshalText implements encoding.TextMarshaler
func (s FramingTrailer) MarshalText() (text []byte, err error) {
	return []byte(s), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *FramingTrailer) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "NUL":
		*s = NULTrailer
	case "LF":
		*s = LFTrailer
	default:
		return fmt.Errorf("unknown syslog format: %s", str)
	}

	return nil
}

// Values taken from tcp input Build function
const tcpDefaultMaxLogSize = helper.ByteSize(stanzainputtcp.DefaultMaxLogSize)
const minMaxLogSize = helper.ByteSize(64 * 1024)

type TCP struct {
	MaxLogSize      units.Base2Bytes            `alloy:"max_log_size,attr,optional"`
	ListenAddress   string                      `alloy:"listen_address,attr,optional"`
	TLS             *otelcol.TLSServerArguments `alloy:"tls,block,optional"`
	AddAttributes   bool                        `alloy:"add_attributes,attr,optional"`
	OneLogPerPacket bool                        `alloy:"one_log_per_packet,attr,optional"`
	Encoding        string                      `alloy:"encoding,attr,optional"`
	MultilineConfig *otelcol.MultilineConfig    `alloy:"multiline,block,optional"`
	TrimConfig      *otelcol.TrimConfig         `alloy:",squash"`
}

type UDP struct {
	ListenAddress   string                   `alloy:"listen_address,attr,optional"`
	OneLogPerPacket bool                     `alloy:"one_log_per_packet,attr,optional"`
	AddAttributes   bool                     `alloy:"add_attributes,attr,optional"`
	Encoding        string                   `alloy:"encoding,attr,optional"`
	MultilineConfig *otelcol.MultilineConfig `alloy:"multiline,block,optional"`
	TrimConfig      *otelcol.TrimConfig      `alloy:",squash"`
	Async           *AsyncConfig             `alloy:"async,block,optional"`
}

type AsyncConfig struct {
	Readers        int `alloy:"readers,attr,optional"`
	Processors     int `alloy:"processors,attr,optional"`
	MaxQueueLength int `alloy:"max_queue_length,attr,optional"`
}

func (c *AsyncConfig) Convert() *stanzainputudp.AsyncConfig {
	if c == nil {
		return nil
	}

	return &stanzainputudp.AsyncConfig{
		Readers:        c.Readers,
		Processors:     c.Processors,
		MaxQueueLength: c.MaxQueueLength,
	}
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Location: "UTC",
		Protocol: config.SyslogFormatRFC5424,
		Output:   &otelcol.ConsumerArguments{},
		OnError:  "send",
	}
	args.DebugMetrics.SetToDefault()
	args.ConsumerRetry.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	c := stanzainputsyslog.NewConfig()
	c.BaseConfig = stanzaparsersyslog.BaseConfig{
		Protocol:            string(args.Protocol),
		Location:            args.Location,
		EnableOctetCounting: args.EnableOctetCounting,
		MaxOctets:           args.MaxOctets,
		AllowSkipPriHeader:  args.AllowSkipPriHeader,
	}

	if args.NonTransparentFramingTrailer != nil {
		s := string(*args.NonTransparentFramingTrailer)
		c.BaseConfig.NonTransparentFramingTrailer = &s
	}

	if args.TCP != nil {
		c.TCP = &stanzainputtcp.BaseConfig{
			MaxLogSize:      helper.ByteSize(args.TCP.MaxLogSize),
			ListenAddress:   args.TCP.ListenAddress,
			AddAttributes:   args.TCP.AddAttributes,
			OneLogPerPacket: args.TCP.OneLogPerPacket,
			Encoding:        args.TCP.Encoding,
		}
		if c.TCP.MaxLogSize == 0 {
			c.TCP.MaxLogSize = tcpDefaultMaxLogSize
		}
		split := args.TCP.MultilineConfig.Convert()
		if split != nil {
			c.TCP.SplitConfig = *split
		}
		trim := args.TCP.TrimConfig.Convert()
		if trim != nil {
			c.TCP.TrimConfig = *trim
		}
		tls := args.TCP.TLS.Convert()
		c.TCP.TLS = tls.Get()
	}

	if args.UDP != nil {
		c.UDP = &stanzainputudp.BaseConfig{
			ListenAddress:   args.UDP.ListenAddress,
			OneLogPerPacket: args.UDP.OneLogPerPacket,
			AddAttributes:   args.UDP.AddAttributes,
			Encoding:        args.UDP.Encoding,
		}
		split := args.UDP.MultilineConfig.Convert()
		if split != nil {
			c.UDP.SplitConfig = *split
		}
		trim := args.UDP.TrimConfig.Convert()
		if trim != nil {
			c.UDP.TrimConfig = *trim
		}
		async := args.UDP.Async.Convert()
		if async != nil {
			c.UDP.AsyncConfig = async
		}
	}

	c.OnError = args.OnError

	def := syslogreceiver.ReceiverType{}.CreateDefaultConfig()
	cfg := def.(*syslogreceiver.SysLogConfig)
	cfg.InputConfig = *c

	// consumerretry package is stanza internal so we can't just Convert
	cfg.RetryOnFailure.Enabled = args.ConsumerRetry.Enabled
	cfg.RetryOnFailure.InitialInterval = args.ConsumerRetry.InitialInterval
	cfg.RetryOnFailure.MaxInterval = args.ConsumerRetry.MaxInterval
	cfg.RetryOnFailure.MaxElapsedTime = args.ConsumerRetry.MaxElapsedTime

	return cfg, nil
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

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	var errs error
	if args.TCP == nil && args.UDP == nil {
		errs = multierror.Append(errs, fmt.Errorf("at least one of 'tcp' or 'udp' must be configured"))
	}

	if args.Protocol != config.SyslogFormatRFC3164 && args.Protocol != config.SyslogFormatRFC5424 {
		errs = multierror.Append(errs, fmt.Errorf("invalid protocol, must be one of 'rfc3164', 'rfc5424': %s", args.Protocol))
	}

	if args.TCP != nil {
		if err := validateListenAddress(args.TCP.ListenAddress, "tcp.listen_address"); err != nil {
			errs = multierror.Append(errs, err)
		}

		if args.NonTransparentFramingTrailer != nil && *args.NonTransparentFramingTrailer != LFTrailer && *args.NonTransparentFramingTrailer != NULTrailer {
			errs = multierror.Append(errs, fmt.Errorf("invalid non_transparent_framing_trailer, must be one of 'LF', 'NUL': %s", *args.NonTransparentFramingTrailer))
		}

		_, err := textutils.LookupEncoding(args.TCP.Encoding)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("invalid tcp.encoding: %w", err))
		}

		if args.TCP.MaxLogSize != 0 && (int64(args.TCP.MaxLogSize) < int64(minMaxLogSize)) {
			errs = multierror.Append(errs, fmt.Errorf("invalid value %d for parameter 'tcp.max_log_size', must be equal to or greater than %d bytes", args.TCP.MaxLogSize, minMaxLogSize))
		}
	}

	if args.UDP != nil {
		if err := validateListenAddress(args.UDP.ListenAddress, "udp.listen_address"); err != nil {
			errs = multierror.Append(errs, err)
		}

		_, err := textutils.LookupEncoding(args.UDP.Encoding)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("invalid udp.encoding: %w", err))
		}
	}

	switch args.OnError {
	case "drop", "drop_quiet", "send", "send_quiet":
	default:
		errs = multierror.Append(errs, fmt.Errorf("invalid on_error: %s", args.OnError))
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
