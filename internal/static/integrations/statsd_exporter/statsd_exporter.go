// Package statsd_exporter embeds https://github.com/prometheus/statsd_exporter
package statsd_exporter

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/statsd_exporter/pkg/address"
	"github.com/prometheus/statsd_exporter/pkg/event"
	"github.com/prometheus/statsd_exporter/pkg/exporter"
	"github.com/prometheus/statsd_exporter/pkg/line"
	"github.com/prometheus/statsd_exporter/pkg/listener"
	"github.com/prometheus/statsd_exporter/pkg/mapper"
	"github.com/prometheus/statsd_exporter/pkg/mappercache/lru"
	"github.com/prometheus/statsd_exporter/pkg/mappercache/randomreplacement"
	"github.com/prometheus/statsd_exporter/pkg/relay"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default settings for the statsd_exporter integration.
var DefaultConfig = Config{
	ListenUDP:      ":9125",
	ListenTCP:      ":9125",
	UnixSocketMode: "755",

	CacheSize:           1000,
	CacheType:           "lru",
	EventQueueSize:      10000,
	EventFlushThreshold: 1000,
	EventFlushInterval:  200 * time.Millisecond,

	ParseDogStatsd: true,
	ParseInfluxDB:  true,
	ParseLibrato:   true,
	ParseSignalFX:  true,

	RelayPacketLength: 1400,
}

// Config controls the statsd_exporter integration.
type Config struct {
	ListenUDP      string `yaml:"listen_udp,omitempty"`
	ListenTCP      string `yaml:"listen_tcp,omitempty"`
	ListenUnixgram string `yaml:"listen_unixgram,omitempty"`
	UnixSocketMode string `yaml:"unix_socket_mode,omitempty"`
	MappingConfig  any    `yaml:"mapping_config,omitempty"`

	ReadBuffer          int           `yaml:"read_buffer,omitempty"`
	CacheSize           int           `yaml:"cache_size,omitempty"`
	CacheType           string        `yaml:"cache_type,omitempty"`
	EventQueueSize      int           `yaml:"event_queue_size,omitempty"`
	EventFlushThreshold int           `yaml:"event_flush_threshold,omitempty"`
	EventFlushInterval  time.Duration `yaml:"event_flush_interval,omitempty"`

	ParseDogStatsd bool `yaml:"parse_dogstatsd_tags,omitempty"`
	ParseInfluxDB  bool `yaml:"parse_influxdb_tags,omitempty"`
	ParseLibrato   bool `yaml:"parse_librato_tags,omitempty"`
	ParseSignalFX  bool `yaml:"parse_signalfx_tags,omitempty"`

	RelayAddr         string `yaml:"relay_addr,omitempty"`
	RelayPacketLength int    `yaml:"relay_packet_length,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "statsd_exporter"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.NewNamedShim("statsd"))
}

// Exporter defines the statsd_exporter integration.
type Exporter struct {
	cfg      *Config
	reg      *prometheus.Registry
	metrics  *Metrics
	exporter *exporter.Exporter
	log      log.Logger
}

// New creates a new statsd_exporter integration. The integration scrapes
// metrics from a statsd process.
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	reg := prometheus.NewRegistry()

	m, err := NewMetrics(reg)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics for network listeners: %w", err)
	}

	if c.ListenUDP == "" && c.ListenTCP == "" && c.ListenUnixgram == "" {
		return nil, fmt.Errorf("at least one of UDP/TCP/Unixgram listeners must be used")
	}
	statsdMapper := &mapper.MetricMapper{
		Registerer:    reg,
		MappingsCount: m.MappingsCount,
		Logger:        log,
	}

	if c.MappingConfig != nil {
		cfgBytes, err := yaml.Marshal(c.MappingConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize mapping config: %w", err)
		}

		err = statsdMapper.InitFromYAMLString(string(cfgBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to load mapping config: %w", err)
		}
	}

	var cache mapper.MetricMapperCache
	if c.CacheSize != 0 {
		switch c.CacheType {
		case "lru":
			cache, err = lru.NewMetricMapperLRUCache(statsdMapper.Registerer, c.CacheSize)
		case "random":
			cache, err = randomreplacement.NewMetricMapperRRCache(statsdMapper.Registerer, c.CacheSize)
		default:
			err = fmt.Errorf("unsupported cache type %q", c.CacheType)
		}
		if err != nil {
			return nil, err
		}
	}
	if cache != nil {
		statsdMapper.UseCache(cache)
	}

	e := exporter.NewExporter(reg, statsdMapper, log, m.EventsActions, m.EventsUnmapped, m.ErrorEventStats, m.EventStats, m.ConflictingEventStats, m.MetricsCount)

	if err := reg.Register(build.NewCollector("statsd_exporter")); err != nil {
		return nil, fmt.Errorf("couldn't register version metrics: %w", err)
	}

	return &Exporter{
		cfg:      c,
		metrics:  m,
		exporter: e,
		reg:      reg,
		log:      log,
	}, nil
}

// MetricsHandler returns the HTTP handler for the integration.
func (e *Exporter) MetricsHandler() (http.Handler, error) {
	return promhttp.HandlerFor(e.reg, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	}), nil
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (e *Exporter) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{JobName: e.cfg.Name(), MetricsPath: "/metrics"}}
}

// Run satisfies Run.
func (e *Exporter) Run(ctx context.Context) error {
	parser := line.NewParser()
	if e.cfg.ParseDogStatsd {
		parser.EnableDogstatsdParsing()
	}
	if e.cfg.ParseInfluxDB {
		parser.EnableInfluxdbParsing()
	}
	if e.cfg.ParseLibrato {
		parser.EnableLibratoParsing()
	}
	if e.cfg.ParseSignalFX {
		parser.EnableSignalFXParsing()
	}

	events := make(chan event.Events, e.cfg.EventQueueSize)
	defer close(events)
	eventQueue := event.NewEventQueue(events, e.cfg.EventFlushThreshold, e.cfg.EventFlushInterval, e.metrics.EventsFlushed)

	var relayTarget *relay.Relay
	if e.cfg.RelayAddr != "" {
		var err error
		relayTarget, err = relay.NewRelay(e.log, e.cfg.RelayAddr, uint(e.cfg.RelayPacketLength))
		if err != nil {
			return fmt.Errorf("failed to create relay: %w", err)
		}
	}

	if e.cfg.ListenUDP != "" {
		addr, err := address.UDPAddrFromString(e.cfg.ListenUDP)
		if err != nil {
			return fmt.Errorf("invalid UDP listen address %s: %w", e.cfg.ListenUDP, err)
		}
		uconn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("failed to start UDP listener: %w", err)
		}
		defer func() {
			err := uconn.Close()
			if err != nil {
				level.Warn(e.log).Log("msg", "failed to close UDP listener", "err", err)
			}
		}()

		if e.cfg.ReadBuffer != 0 {
			err = uconn.SetReadBuffer(e.cfg.ReadBuffer)
			if err != nil {
				return fmt.Errorf("failed to set UDP read buffer: %w", err)
			}
		}

		ul := &listener.StatsDUDPListener{
			Conn:            uconn,
			EventHandler:    eventQueue,
			Relay:           relayTarget,
			Logger:          e.log,
			LineParser:      parser,
			UDPPackets:      e.metrics.UDPPackets,
			LinesReceived:   e.metrics.LinesReceived,
			EventsFlushed:   e.metrics.EventsFlushed,
			SampleErrors:    *e.metrics.SampleErrors,
			SamplesReceived: e.metrics.SamplesReceived,
			TagErrors:       e.metrics.TagErrors,
			TagsReceived:    e.metrics.TagsReceived,
		}

		go ul.Listen()
	}

	if e.cfg.ListenTCP != "" {
		addr, err := address.TCPAddrFromString(e.cfg.ListenTCP)
		if err != nil {
			return fmt.Errorf("invalid TCP listen address %s: %w", e.cfg.ListenTCP, err)
		}
		tconn, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to start TCP listener: %w", err)
		}
		defer func() {
			err := tconn.Close()
			if err != nil {
				level.Warn(e.log).Log("msg", "failed to close TCP listener", "err", err)
			}
		}()

		tl := &listener.StatsDTCPListener{
			Conn:            tconn,
			EventHandler:    eventQueue,
			Relay:           relayTarget,
			Logger:          e.log,
			LineParser:      parser,
			LinesReceived:   e.metrics.LinesReceived,
			EventsFlushed:   e.metrics.EventsFlushed,
			SampleErrors:    *e.metrics.SampleErrors,
			SamplesReceived: e.metrics.SamplesReceived,
			TagErrors:       e.metrics.TagErrors,
			TagsReceived:    e.metrics.TagsReceived,
			TCPConnections:  e.metrics.TCPConnections,
			TCPErrors:       e.metrics.TCPErrors,
			TCPLineTooLong:  e.metrics.TCPLineTooLong,
		}

		go tl.Listen()
	}

	if e.cfg.ListenUnixgram != "" {
		var err error
		if _, err = os.Stat(e.cfg.ListenUnixgram); !os.IsNotExist(err) {
			return fmt.Errorf("unixgram socket %s already exists: %w", e.cfg.ListenUnixgram, err)
		}
		uxgconn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
			Net:  "unixgram",
			Name: e.cfg.ListenUnixgram,
		})
		if err != nil {
			return fmt.Errorf("failed to listen on unixgram socket: %w", err)
		}
		defer func() {
			err := uxgconn.Close()
			if err != nil {
				level.Warn(e.log).Log("msg", "failed to close unixgram listener", "err", err)
			}
		}()

		if e.cfg.ReadBuffer != 0 {
			err = uxgconn.SetReadBuffer(e.cfg.ReadBuffer)
			if err != nil {
				return fmt.Errorf("error setting unixgram read buffer: %w", err)
			}
		}

		ul := &listener.StatsDUnixgramListener{
			Conn:            uxgconn,
			EventHandler:    eventQueue,
			Logger:          e.log,
			Relay:           relayTarget,
			LineParser:      parser,
			UnixgramPackets: e.metrics.UnixgramPackets,
			LinesReceived:   e.metrics.LinesReceived,
			EventsFlushed:   e.metrics.EventsFlushed,
			SampleErrors:    *e.metrics.SampleErrors,
			SamplesReceived: e.metrics.SamplesReceived,
			TagErrors:       e.metrics.TagErrors,
			TagsReceived:    e.metrics.TagsReceived,
		}

		go ul.Listen()

		// If it's an abstract unix domain socket, it won't exist on fs so we can't
		// chmod it either.
		if _, err := os.Stat(e.cfg.ListenUnixgram); !os.IsNotExist(err) {
			defer os.Remove(e.cfg.ListenUnixgram)

			// Convert the string to octet
			perm, err := strconv.ParseInt("0"+e.cfg.UnixSocketMode, 8, 32)
			if err != nil {
				level.Warn(e.log).Log("msg", "bad permission on unixgram socket, ignoring", "permission", e.cfg.UnixSocketMode, "socket", e.cfg.ListenUnixgram, "err", err)
			} else {
				err = os.Chmod(e.cfg.ListenUnixgram, os.FileMode(perm))
				if err != nil {
					level.Warn(e.log).Log("msg", "failed to change unixgram socket permission", "socket", e.cfg.ListenUnixgram, "err", err)
				}
			}
		}
	}

	go e.exporter.Listen(events)

	<-ctx.Done()
	return nil
}
