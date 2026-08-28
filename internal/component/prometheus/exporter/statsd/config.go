package statsd

import (
	"fmt"
	"os"
	"time"

	"github.com/grafana/alloy/internal/static/integrations/statsd_exporter"
	"gopkg.in/yaml.v3"
)

type Arguments struct {
	ListenUDP      string `alloy:"listen_udp,attr,optional"`
	ListenTCP      string `alloy:"listen_tcp,attr,optional"`
	ListenUnixgram string `alloy:"listen_unixgram,attr,optional"`
	UnixSocketMode string `alloy:"unix_socket_mode,attr,optional"`
	MappingConfig  string `alloy:"mapping_config_path,attr,optional"`

	ReadBuffer          int           `alloy:"read_buffer,attr,optional"`
	CacheSize           int           `alloy:"cache_size,attr,optional"`
	CacheType           string        `alloy:"cache_type,attr,optional"`
	EventQueueSize      int           `alloy:"event_queue_size,attr,optional"`
	EventFlushThreshold int           `alloy:"event_flush_threshold,attr,optional"`
	EventFlushInterval  time.Duration `alloy:"event_flush_interval,attr,optional"`

	ParseDogStatsd bool `alloy:"parse_dogstatsd_tags,attr,optional"`
	ParseInfluxDB  bool `alloy:"parse_influxdb_tags,attr,optional"`
	ParseLibrato   bool `alloy:"parse_librato_tags,attr,optional"`
	ParseSignalFX  bool `alloy:"parse_signalfx_tags,attr,optional"`

	RelayAddr         string `alloy:"relay_addr,attr,optional"`
	RelayPacketLength int    `alloy:"relay_packet_length,attr,optional"`
}

// DefaultConfig holds non-zero default options for the Config when it is
// unmarshaled from YAML.
//
// Some defaults are populated from init functions in the github.com/grafana/alloy/internal/static/integrations/statsd_exporter package.
var DefaultConfig = Arguments{

	ListenUDP:      statsd_exporter.DefaultConfig.ListenUDP,
	ListenTCP:      statsd_exporter.DefaultConfig.ListenTCP,
	UnixSocketMode: statsd_exporter.DefaultConfig.UnixSocketMode,

	CacheSize:           statsd_exporter.DefaultConfig.CacheSize,
	CacheType:           statsd_exporter.DefaultConfig.CacheType,
	EventQueueSize:      statsd_exporter.DefaultConfig.EventQueueSize,
	EventFlushThreshold: statsd_exporter.DefaultConfig.EventFlushThreshold,
	EventFlushInterval:  statsd_exporter.DefaultConfig.EventFlushInterval,

	ParseDogStatsd: statsd_exporter.DefaultConfig.ParseDogStatsd,
	ParseInfluxDB:  statsd_exporter.DefaultConfig.ParseInfluxDB,
	ParseLibrato:   statsd_exporter.DefaultConfig.ParseLibrato,
	ParseSignalFX:  statsd_exporter.DefaultConfig.ParseSignalFX,

	RelayPacketLength: statsd_exporter.DefaultConfig.RelayPacketLength,
}

// Convert gives a config suitable for use with github.com/grafana/alloy/internal/static/integrations/statsd_exporter.
func (c *Arguments) Convert() (*statsd_exporter.Config, error) {
	var (
		mappingConfig any
		err           error
	)

	if c.MappingConfig != "" {
		mappingConfig, err = readMappingFile(c.MappingConfig)

		if err != nil {
			return nil, fmt.Errorf("failed to convert statsd config: %w", err)
		}
	}

	return &statsd_exporter.Config{
		ListenUDP:           c.ListenUDP,
		ListenTCP:           c.ListenTCP,
		ListenUnixgram:      c.ListenUnixgram,
		UnixSocketMode:      c.UnixSocketMode,
		ReadBuffer:          c.ReadBuffer,
		CacheSize:           c.CacheSize,
		CacheType:           c.CacheType,
		EventQueueSize:      c.EventQueueSize,
		EventFlushThreshold: c.EventFlushThreshold,
		EventFlushInterval:  c.EventFlushInterval,
		ParseDogStatsd:      c.ParseDogStatsd,
		ParseInfluxDB:       c.ParseInfluxDB,
		ParseLibrato:        c.ParseLibrato,
		ParseSignalFX:       c.ParseSignalFX,
		RelayAddr:           c.RelayAddr,
		RelayPacketLength:   c.RelayPacketLength,
		MappingConfig:       mappingConfig,
	}, nil
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultConfig
}

func readMappingFile(path string) (any, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read mapping config file: %w", err)
	}

	var statsdMapper any
	err = yaml.Unmarshal(file, &statsdMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to load mapping config: %w", err)
	}

	return &statsdMapper, nil
}
