package statsd_exporter //nolint:golint

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds metrics used by the statsd_exporter integration. These metrics
// are distinct from the set of metrics used by the exporter itself, and are just
// used to monitor the stats of the listeners that forward data to the exporter.
type Metrics struct {
	EventStats            *prometheus.CounterVec
	EventsFlushed         prometheus.Counter
	EventsUnmapped        prometheus.Counter
	UDPPackets            prometheus.Counter
	TCPConnections        prometheus.Counter
	TCPErrors             prometheus.Counter
	TCPLineTooLong        prometheus.Counter
	UnixgramPackets       prometheus.Counter
	LinesReceived         prometheus.Counter
	SamplesReceived       prometheus.Counter
	SampleErrors          *prometheus.CounterVec
	TagsReceived          prometheus.Counter
	TagErrors             prometheus.Counter
	MappingsCount         prometheus.Gauge
	ConflictingEventStats *prometheus.CounterVec
	ErrorEventStats       *prometheus.CounterVec
	EventsActions         *prometheus.CounterVec
	MetricsCount          *prometheus.GaugeVec
}

// NewMetrics initializes Metrics and registers them to the given Registerer.
func NewMetrics(r prometheus.Registerer) (*Metrics, error) {
	var m Metrics

	m.EventStats = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "statsd_exporter_events_total",
		Help: "The total number of StatsD events seen.",
	}, []string{"type"})
	m.EventsFlushed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_event_queue_flushed_total",
		Help: "Number of times events were flushed to exporter",
	})
	m.EventsUnmapped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_events_unmapped_total",
		Help: "The total number of StatsD events no mapping was found for.",
	})
	m.UDPPackets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_udp_packets_total",
		Help: "The total number of StatsD packets received over UDP.",
	})
	m.TCPConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_tcp_connections_total",
		Help: "The total number of TCP connections handled.",
	})
	m.TCPErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_tcp_connection_errors_total",
		Help: "The number of errors encountered reading from TCP.",
	})
	m.TCPLineTooLong = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_tcp_too_long_lines_total",
		Help: "The number of lines discarded due to being too long.",
	})
	m.UnixgramPackets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_unixgram_packets_total",
		Help: "The total number of StatsD packets received over Unixgram.",
	})
	m.LinesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_lines_total",
		Help: "The total number of StatsD lines received.",
	})
	m.SamplesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_samples_total",
		Help: "The total number of StatsD samples received.",
	})
	m.SampleErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "statsd_exporter_sample_errors_total",
		Help: "The total number of errors parsing StatsD samples.",
	}, []string{"reason"})
	m.TagsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_tags_total",
		Help: "The total number of DogStatsD tags processed.",
	})
	m.TagErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "statsd_exporter_tag_errors_total",
		Help: "The number of errors parsing DogStatsD tags.",
	})
	m.MappingsCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "statsd_exporter_loaded_mappings",
		Help: "The current number of configured metric mappings.",
	})
	m.ConflictingEventStats = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "statsd_exporter_events_conflict_total",
		Help: "The total number of StatsD events with conflicting names.",
	}, []string{"type"})
	m.ErrorEventStats = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "statsd_exporter_events_error_total",
		Help: "The total number of StatsD events discarded due to errors.",
	}, []string{"reason"})
	m.EventsActions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "statsd_exporter_events_actions_total",
		Help: "The total number of StatsD events by action.",
	}, []string{"action"})
	m.MetricsCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "statsd_exporter_metrics_total",
		Help: "The total number of metrics.",
	}, []string{"type"})

	cs := []prometheus.Collector{
		m.EventStats,
		m.EventsFlushed,
		m.EventsUnmapped,
		m.UDPPackets,
		m.TCPConnections,
		m.TCPErrors,
		m.TCPLineTooLong,
		m.UnixgramPackets,
		m.LinesReceived,
		m.SamplesReceived,
		m.SampleErrors,
		m.TagsReceived,
		m.TagErrors,
		m.MappingsCount,
		m.ConflictingEventStats,
		m.ErrorEventStats,
		m.EventsActions,
		m.MetricsCount,
	}
	if r != nil {
		for _, c := range cs {
			if err := r.Register(c); err != nil {
				return nil, err
			}
		}
	}

	return &m, nil
}
