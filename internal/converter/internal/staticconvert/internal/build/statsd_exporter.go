package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/statsd"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/static/integrations/statsd_exporter"
)

func (b *ConfigBuilder) appendStatsdExporter(config *statsd_exporter.Config, instanceKey *string) discovery.Exports {
	args := toStatsdExporter(config)

	if config.MappingConfig != nil {
		b.diags.Add(diag.SeverityLevelError, "mapping_config is not supported in statsd_exporter integrations config")
	}

	return b.appendExporterBlock(args, config.Name(), instanceKey, "statsd")
}

func toStatsdExporter(config *statsd_exporter.Config) *statsd.Arguments {
	return &statsd.Arguments{
		ListenUDP:           config.ListenUDP,
		ListenTCP:           config.ListenTCP,
		ListenUnixgram:      config.ListenUnixgram,
		UnixSocketMode:      config.UnixSocketMode,
		MappingConfig:       "",
		ReadBuffer:          config.ReadBuffer,
		CacheSize:           config.CacheSize,
		CacheType:           config.CacheType,
		EventQueueSize:      config.EventQueueSize,
		EventFlushThreshold: config.EventFlushThreshold,
		EventFlushInterval:  config.EventFlushInterval,
		ParseDogStatsd:      config.ParseDogStatsd,
		ParseInfluxDB:       config.ParseInfluxDB,
		ParseLibrato:        config.ParseLibrato,
		ParseSignalFX:       config.ParseSignalFX,
		RelayAddr:           config.RelayAddr,
		RelayPacketLength:   config.RelayPacketLength,
	}
}
