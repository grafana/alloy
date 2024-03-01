package statsd

import (
	"fmt"
	"os"
	"time"

	"github.com/grafana/agent/internal/static/integrations/statsd_exporter"
	"gopkg.in/yaml.v3"
)

type Arguments struct {
	ListenUDP      string `river:"listen_udp,attr,optional"`
	ListenTCP      string `river:"listen_tcp,attr,optional"`
	ListenUnixgram string `river:"listen_unixgram,attr,optional"`
	UnixSocketMode string `river:"unix_socket_mode,attr,optional"`
	MappingConfig  string `river:"mapping_config_path,attr,optional"`

	ReadBuffer          int           `river:"read_buffer,attr,optional"`
	CacheSize           int           `river:"cache_size,attr,optional"`
	CacheType           string        `river:"cache_type,attr,optional"`
	EventQueueSize      int           `river:"event_queue_size,attr,optional"`
	EventFlushThreshold int           `river:"event_flush_threshold,attr,optional"`
	EventFlushInterval  time.Duration `river:"event_flush_interval,attr,optional"`

	ParseDogStatsd bool `river:"parse_dogstatsd_tags,attr,optional"`
	ParseInfluxDB  bool `river:"parse_influxdb_tags,attr,optional"`
	ParseLibrato   bool `river:"parse_librato_tags,attr,optional"`
	ParseSignalFX  bool `river:"parse_signalfx_tags,attr,optional"`

	RelayAddr         string `river:"relay_addr,attr,optional"`
	RelayPacketLength int    `river:"relay_packet_length,attr,optional"`
}

// DefaultConfig holds non-zero default options for the Config when it is
// unmarshaled from YAML.
//
// Some defaults are populated from init functions in the github.com/grafana/agent/internal/static/integrations/statsd_exporter package.
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

// Convert gives a config suitable for use with github.com/grafana/agent/internal/static/integrations/statsd_exporter.
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

// SetToDefault implements river.Defaulter.
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
