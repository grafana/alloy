package types

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// TODO @mattdurham separate this into more manageable chunks, and likely 3 stats series: series, metadata and new ones.

type SerializerStats struct {
	SeriesStored    int
	MetadataStored  int
	Errors          int
	NewestTimestamp int64
}

type PrometheusStats struct {
	register prometheus.Registerer
	// Network Stats
	NetworkSeriesSent                prometheus.Counter
	NetworkFailures                  prometheus.Counter
	NetworkRetries                   prometheus.Counter
	NetworkRetries429                prometheus.Counter
	NetworkRetries5XX                prometheus.Counter
	NetworkSentDuration              prometheus.Histogram
	NetworkErrors                    prometheus.Counter
	NetworkNewestOutTimeStampSeconds prometheus.Gauge

	// Serializer Stats
	SerializerInSeries                 prometheus.Counter
	SerializerNewestInTimeStampSeconds prometheus.Gauge
	SerializerErrors                   prometheus.Counter

	// Backwards compatibility metrics
	SamplesTotal    prometheus.Counter
	HistogramsTotal prometheus.Counter
	MetadataTotal   prometheus.Counter

	FailedSamplesTotal    prometheus.Counter
	FailedHistogramsTotal prometheus.Counter
	FailedMetadataTotal   prometheus.Counter

	RetriedSamplesTotal    prometheus.Counter
	RetriedHistogramsTotal prometheus.Counter
	RetriedMetadataTotal   prometheus.Counter

	EnqueueRetriesTotal  prometheus.Counter
	SentBatchDuration    prometheus.Histogram
	HighestSentTimestamp prometheus.Gauge

	SentBytesTotal            prometheus.Counter
	MetadataBytesTotal        prometheus.Counter
	RemoteStorageInTimestamp  prometheus.Gauge
	RemoteStorageOutTimestamp prometheus.Gauge
	RemoteStorageDuration     prometheus.Histogram
}

func NewStats(namespace, subsystem string, registry prometheus.Registerer) *PrometheusStats {
	s := &PrometheusStats{
		register: registry,
		SerializerInSeries: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "serializer_incoming_signals",
		}),
		SerializerNewestInTimeStampSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "serializer_incoming_timestamp_seconds",
		}),
		SerializerErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "serializer_errors",
		}),
		NetworkNewestOutTimeStampSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_timestamp_seconds",
		}),
		RemoteStorageDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "prometheus_remote_storage_queue_duration_seconds",
		}),
		NetworkSeriesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_sent",
		}),
		NetworkFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_failed",
		}),
		NetworkRetries: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_retried",
		}),
		NetworkRetries429: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_retried_429",
		}),
		NetworkRetries5XX: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_retried_5xx",
		}),
		NetworkSentDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace:                   namespace,
			Subsystem:                   subsystem,
			Name:                        "network_duration_seconds",
			NativeHistogramBucketFactor: 1.1,
		}),
		NetworkErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "network_errors",
		}),
		RemoteStorageOutTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "prometheus_remote_storage_queue_highest_sent_timestamp_seconds",
		}),
		RemoteStorageInTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "prometheus_remote_storage_highest_timestamp_in_seconds",
		}),
		SamplesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_samples_total",
			Help: "Total number of samples sent to remote storage.",
		}),
		HistogramsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_histograms_total",
			Help: "Total number of histograms sent to remote storage.",
		}),
		MetadataTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_metadata_total",
			Help: "Total number of metadata sent to remote storage.",
		}),
		FailedSamplesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_samples_failed_total",
			Help: "Total number of samples which failed on send to remote storage, non-recoverable errors.",
		}),
		FailedHistogramsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_histograms_failed_total",
			Help: "Total number of histograms which failed on send to remote storage, non-recoverable errors.",
		}),
		FailedMetadataTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_metadata_failed_total",
			Help: "Total number of metadata entries which failed on send to remote storage, non-recoverable errors.",
		}),

		RetriedSamplesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_samples_retried_total",
			Help: "Total number of samples which failed on send to remote storage but were retried because the send error was recoverable.",
		}),
		RetriedHistogramsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_histograms_retried_total",
			Help: "Total number of histograms which failed on send to remote storage but were retried because the send error was recoverable.",
		}),
		RetriedMetadataTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_metadata_retried_total",
			Help: "Total number of metadata entries which failed on send to remote storage but were retried because the send error was recoverable.",
		}),
		SentBytesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_sent_bytes_total",
			Help: "The total number of bytes of data (not metadata) sent by the queue after compression. Note that when exemplars over remote write is enabled the exemplars included in a remote write request count towards this metric.",
		}),
		MetadataBytesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_remote_storage_metadata_bytes_total",
			Help: "The total number of bytes of metadata sent by the queue after compression.",
		}),
	}
	registry.MustRegister(
		s.NetworkSentDuration,
		s.NetworkRetries5XX,
		s.NetworkRetries429,
		s.NetworkRetries,
		s.NetworkFailures,
		s.NetworkSeriesSent,
		s.NetworkErrors,
		s.NetworkNewestOutTimeStampSeconds,
		s.SerializerInSeries,
		s.SerializerErrors,
		s.SerializerNewestInTimeStampSeconds,
	)
	return s
}

func (s *PrometheusStats) Unregister() {
	unregistered := []prometheus.Collector{
		s.RemoteStorageDuration,
		s.RemoteStorageInTimestamp,
		s.RemoteStorageOutTimestamp,
		s.SamplesTotal,
		s.HistogramsTotal,
		s.FailedSamplesTotal,
		s.FailedHistogramsTotal,
		s.RetriedSamplesTotal,
		s.RetriedHistogramsTotal,
		s.SentBytesTotal,
		s.MetadataTotal,
		s.FailedMetadataTotal,
		s.RetriedMetadataTotal,
		s.MetadataBytesTotal,
		s.NetworkSentDuration,
		s.NetworkRetries5XX,
		s.NetworkRetries429,
		s.NetworkRetries,
		s.NetworkFailures,
		s.NetworkSeriesSent,
		s.NetworkErrors,
		s.NetworkNewestOutTimeStampSeconds,
		s.SerializerInSeries,
		s.SerializerErrors,
		s.SerializerNewestInTimeStampSeconds,
	}

	for _, g := range unregistered {
		s.register.Unregister(g)
	}

}

func (s *PrometheusStats) SeriesBackwardsCompatibility(registry prometheus.Registerer) {
	registry.MustRegister(
		s.RemoteStorageDuration,
		s.RemoteStorageInTimestamp,
		s.RemoteStorageOutTimestamp,
		s.SamplesTotal,
		s.HistogramsTotal,
		s.FailedSamplesTotal,
		s.FailedHistogramsTotal,
		s.RetriedSamplesTotal,
		s.RetriedHistogramsTotal,
		s.SentBytesTotal,
	)
}

func (s *PrometheusStats) MetaBackwardsCompatibility(registry prometheus.Registerer) {
	registry.MustRegister(
		s.MetadataTotal,
		s.FailedMetadataTotal,
		s.RetriedMetadataTotal,
		s.MetadataBytesTotal,
	)
}

func (s *PrometheusStats) UpdateNetwork(stats NetworkStats) {
	s.NetworkSeriesSent.Add(float64(stats.TotalSent()))
	s.NetworkRetries.Add(float64(stats.TotalRetried()))
	s.NetworkFailures.Add(float64(stats.TotalFailed()))
	s.NetworkRetries429.Add(float64(stats.Total429()))
	s.NetworkRetries5XX.Add(float64(stats.Total5XX()))
	s.NetworkSentDuration.Observe(stats.SendDuration.Seconds())
	s.RemoteStorageDuration.Observe(stats.SendDuration.Seconds())
	// The newest timestamp is no always sent.
	if stats.NewestTimestamp != 0 {
		s.RemoteStorageOutTimestamp.Set(float64(stats.NewestTimestamp))
		s.NetworkNewestOutTimeStampSeconds.Set(float64(stats.NewestTimestamp))
	}

	s.SamplesTotal.Add(float64(stats.Series.SeriesSent))
	s.MetadataTotal.Add(float64(stats.Metadata.SeriesSent))
	s.HistogramsTotal.Add(float64(stats.Histogram.SeriesSent))

	s.FailedSamplesTotal.Add(float64(stats.Series.FailedSamples))
	s.FailedMetadataTotal.Add(float64(stats.Metadata.FailedSamples))
	s.FailedHistogramsTotal.Add(float64(stats.Histogram.FailedSamples))

	s.RetriedSamplesTotal.Add(float64(stats.Series.RetriedSamples))
	s.RetriedHistogramsTotal.Add(float64(stats.Histogram.RetriedSamples))
	s.RetriedMetadataTotal.Add(float64(stats.Metadata.RetriedSamples))

	s.MetadataBytesTotal.Add(float64(stats.MetadataBytes))
	s.SentBytesTotal.Add(float64(stats.SeriesBytes))
}

func (s *PrometheusStats) UpdateSerializer(stats SerializerStats) {
	s.SerializerInSeries.Add(float64(stats.SeriesStored))
	s.SerializerInSeries.Add(float64(stats.MetadataStored))
	s.SerializerErrors.Add(float64(stats.Errors))
	if stats.NewestTimestamp != 0 {
		s.SerializerNewestInTimeStampSeconds.Set(float64(stats.NewestTimestamp))
		s.RemoteStorageInTimestamp.Set(float64(stats.NewestTimestamp))
	}

}

type NetworkStats struct {
	Series          CategoryStats
	Histogram       CategoryStats
	Metadata        CategoryStats
	SendDuration    time.Duration
	NewestTimestamp int64
	SeriesBytes     int
	MetadataBytes   int
}

func (ns NetworkStats) TotalSent() int {
	return ns.Series.SeriesSent + ns.Histogram.SeriesSent + ns.Metadata.SeriesSent
}

func (ns NetworkStats) TotalRetried() int {
	return ns.Series.RetriedSamples + ns.Histogram.RetriedSamples + ns.Metadata.RetriedSamples
}

func (ns NetworkStats) TotalFailed() int {
	return ns.Series.FailedSamples + ns.Histogram.FailedSamples + ns.Metadata.FailedSamples
}

func (ns NetworkStats) Total429() int {
	return ns.Series.RetriedSamples429 + ns.Histogram.RetriedSamples429 + ns.Metadata.RetriedSamples429
}

func (ns NetworkStats) Total5XX() int {
	return ns.Series.RetriedSamples5XX + ns.Histogram.RetriedSamples5XX + ns.Metadata.RetriedSamples5XX
}

type CategoryStats struct {
	RetriedSamples       int
	RetriedSamples429    int
	RetriedSamples5XX    int
	SeriesSent           int
	FailedSamples        int
	NetworkSamplesFailed int
}
