package convert

import (
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"
)

// memorySeries is an in-memory series mapped from an OpenTelemetry Collector
// data point.
type memorySeries struct {
	// We shouldn't need an RWMutex here because there should only ever be
	// exactly one goroutine for each memory series, since each series is
	// intended to be unique.
	sync.Mutex

	labels   labels.Labels     // Labels used for writing.
	metadata map[string]string // Extra (optional) metadata used for conversion.

	id storage.SeriesRef // id returned by storage.Appender.

	timestamp time.Time // Timestamp used for out-of-order detection.
	lastSeen  time.Time // Timestamp used for garbage collection.

	value float64 // Value used for writing.
}

func newMemorySeries(metadata map[string]string, labels labels.Labels) *memorySeries {
	return &memorySeries{
		metadata: metadata,
		labels:   labels,
	}
}

// Metadata returns a metadata value by key.
func (series *memorySeries) Metadata(key string) string {
	if series.metadata == nil {
		return ""
	}
	return series.metadata[key]
}

// Timestamp returns the current timestamp of this series.
func (series *memorySeries) Timestamp() time.Time {
	series.Lock()
	defer series.Unlock()
	return series.timestamp
}

// SetTimestamp updates the current timestamp of this series.
func (series *memorySeries) SetTimestamp(newTime time.Time) {
	// TODO(rfratto): does this need to be a CAS-style function instead?
	series.Lock()
	defer series.Unlock()
	series.timestamp = newTime
}

// LastSeen returns the timestamp when this series was last seen.
func (series *memorySeries) LastSeen() time.Time {
	series.Lock()
	defer series.Unlock()
	return series.lastSeen
}

// Ping updates the last seen timestamp of this series.
func (series *memorySeries) Ping() {
	series.Lock()
	defer series.Unlock()
	series.lastSeen = time.Now()
}

// Value gets the current value of this series.
func (series *memorySeries) Value() float64 {
	series.Lock()
	defer series.Unlock()
	return series.value
}

// SetValue updates the current value of this series.
func (series *memorySeries) SetValue(newValue float64) {
	// TODO(rfratto): does this need to be a CAS-style function instead?
	series.Lock()
	defer series.Unlock()
	series.value = newValue
}

func (series *memorySeries) WriteTo(app storage.Appender, ts time.Time) error {
	series.Lock()
	defer series.Unlock()

	newID, err := app.Append(series.id, series.labels, timestamp.FromTime(ts), series.value)
	if err != nil {
		return err
	}

	if newID != series.id {
		series.id = newID
	}

	return nil
}

func (series *memorySeries) WriteExemplarsTo(app storage.Appender, e exemplar.Exemplar) error {
	series.Lock()
	defer series.Unlock()

	if _, err := app.AppendExemplar(series.id, series.labels, e); err != nil {
		return err
	}

	return nil
}

func (series *memorySeries) WriteNativeHistogramTo(app storage.Appender, ts time.Time, h *histogram.Histogram, fh *histogram.FloatHistogram) error {
	series.Lock()
	defer series.Unlock()

	if _, err := app.AppendHistogram(series.id, series.labels, timestamp.FromTime(ts), h, fh); err != nil {
		return err
	}

	return nil
}

type memoryMetadata struct {
	sync.Mutex

	// ID returned by the underlying storage.Appender.
	ID   storage.SeriesRef
	Name string

	lastSeen time.Time
	metadata metadata.Metadata

	// Used for determining when a write needs to occur.
	lastWrite, lastUpdate time.Time
}

// WriteTo writes the metadata to app if the metadata has changed since the
// last update, otherwise WriteTo is a no-op.
func (md *memoryMetadata) WriteTo(app storage.Appender, ts time.Time) error {
	md.Lock()
	defer md.Unlock()

	if !md.lastWrite.Before(md.lastUpdate) {
		return nil
	}

	labels := labels.FromStrings(model.MetricNameLabel, md.Name)

	ref, err := app.UpdateMetadata(md.ID, labels, md.metadata)
	if err != nil {
		return err
	}
	if ref != md.ID {
		md.ID = ref
	}

	md.lastWrite = md.lastUpdate
	return nil
}

// Update updates the metadata used by md. The next call to WriteTo will write
// the new metadata only if m is different from the last metadata stored.
func (md *memoryMetadata) Update(m metadata.Metadata) {
	md.Lock()
	defer md.Unlock()

	md.lastSeen = time.Now()

	// Metadata hasn't changed; don't do anything.
	if m == md.metadata {
		return
	}

	md.metadata = m
	md.lastUpdate = time.Now()
}
