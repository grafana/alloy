package prometheus

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/service/labelstore"
)

var _ storage.Appendable = (*Fanout)(nil)

// Fanout supports the default Alloy style of appendables since it can go to multiple outputs. It also allows the intercepting of appends.
// It also maintains the responsibility of assigning global ref IDs to a series via the label store.
type Fanout struct {
	mut sync.RWMutex
	// children is where to fan out.
	children []storage.Appendable
	// ComponentID is what component this belongs to.
	componentID    string
	writeLatency   prometheus.Histogram
	samplesCounter prometheus.Counter
	ls             labelstore.LabelStore

	// lastSeriesCount stores the number of series that were sent through the last appender. It helps to estimate how
	// much memory to allocate for the staleness trackers.
	lastSeriesCount atomic.Int64

	useLabelStore         bool
	seriesRefMappingStore *appenders.SeriesRefMappingStore
	// deadRefThreshold is updated on every children change. Any store-issued ref
	// below this value was issued before the last topology change and must be
	// zeroed before forwarding to a child appender.
	deadRefThreshold storage.SeriesRef
}

func normalizeChildren(children []storage.Appendable) []storage.Appendable {
	if len(children) == 0 {
		return nil
	}

	cloned := slices.Clone(children)
	return slices.DeleteFunc(cloned, func(i storage.Appendable) bool { return i == nil })
}

// NewFanout creates a fanout appendable.
func NewFanout(children []storage.Appendable, componentID string, register prometheus.Registerer, ls labelstore.LabelStore) *Fanout {
	wl := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "prometheus_fanout_latency",
		Help:    "Write latency for sending to direct and indirect components",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
	})
	_ = register.Register(wl)

	// Note: this only covers calls to Append. It could make more sense when upstream changes to AppendV2 where there will be
	// only a single Append function. But we might want to make this a CounterVec with different labels for the
	// different appended types.
	s := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_forwarded_samples_total",
		Help: "Total number of samples sent to downstream components.",
	})
	_ = register.Register(s)

	useLabelStore := ls != nil && ls.Enabled()

	return &Fanout{
		children:       normalizeChildren(children),
		componentID:    componentID,
		writeLatency:   wl,
		samplesCounter: s,
		ls:             ls,

		useLabelStore:         useLabelStore,
		seriesRefMappingStore: appenders.NewSeriesRefMappingStore(register),
	}
}

// UpdateChildren allows changing of the children of the fanout.
//
// When children change, the store is cleared to start a new ref generation.
// This guards against two ref collision hazards that arise when the appender
// type changes between passthrough (1 child) and seriesRefMapping (N children):
//
// passthrough → seriesRefMapping: a cached raw child ref (e.g. WAL ref) may collide
// numerically with a store-issued unique ref for a different series. The store guards
// against this with a label hash check on every mapping lookup.
//
// seriesRefMapping → passthrough: a cached store-issued unique ref is meaningless to
// the child and must not be forwarded. Clear returns the new generation boundary;
// the passthrough zeros any ref below it before forwarding.
func (f *Fanout) UpdateChildren(children []storage.Appendable) {
	c := normalizeChildren(children)

	f.mut.Lock()
	defer f.mut.Unlock()

	if !slices.Equal(f.children, c) {
		f.deadRefThreshold = f.seriesRefMappingStore.Clear()
	}
	f.children = c
}

// Appender satisfies the Appendable interface.
func (f *Fanout) Appender(ctx context.Context) storage.Appender {
	f.mut.RLock()
	defer f.mut.RUnlock()

	// We should only change the context if
	// it already doesn't have a target or metadata store.
	// It will have these if the scrape loop was started
	// with the PassMetadataInContext option set to true.
	t, ok := scrape.TargetFromContext(ctx)
	if !ok || t == nil {
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
	}

	s, ok := scrape.MetricMetadataStoreFromContext(ctx)
	if !ok || s == nil {
		ctx = scrape.ContextWithMetricMetadataStore(ctx, NoopMetadataStore{})
	}

	children := make([]storage.Appender, 0, len(f.children))
	for _, c := range f.children {
		children = append(children, c.Appender(ctx))
	}

	if f.useLabelStore {
		return &appender{
			children:          children,
			fanout:            f,
			stalenessTrackers: make([]labelstore.StalenessTracker, 0, f.lastSeriesCount.Load()),
		}
	}

	return appenders.New(children, f.seriesRefMappingStore, f.deadRefThreshold, f.writeLatency, f.samplesCounter)
}

func (f *Fanout) Clear() {
	f.mut.Lock()
	defer f.mut.Unlock()

	f.deadRefThreshold = f.seriesRefMappingStore.Clear()
}

type appender struct {
	children          []storage.Appender
	start             time.Time
	stalenessTrackers []labelstore.StalenessTracker
	fanout            *Fanout
}

func (a *appender) SetOptions(opts *storage.AppendOptions) {
	for _, x := range a.children {
		x.SetOptions(opts)
	}
}

var _ storage.Appender = (*appender)(nil)

// Append satisfies the Appender interface.
func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	a.stalenessTrackers = append(a.stalenessTrackers, labelstore.StalenessTracker{
		GlobalRefID: uint64(ref),
		Labels:      l,
		Value:       v,
	})
	var multiErr error
	updated := false
	for _, x := range a.children {
		_, err := x.Append(ref, l, t, v)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		} else {
			updated = true
		}
	}
	if updated {
		a.fanout.samplesCounter.Inc()
	}
	return ref, multiErr
}

// Commit satisfies the Appender interface.
func (a *appender) Commit() error {
	defer a.recordLatency()
	var multiErr error
	a.fanout.lastSeriesCount.Store(int64(len(a.stalenessTrackers)))
	a.fanout.ls.TrackStaleness(a.stalenessTrackers)
	for _, x := range a.children {
		err := x.Commit()
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

// Rollback satisfies the Appender interface.
func (a *appender) Rollback() error {
	defer a.recordLatency()
	a.fanout.lastSeriesCount.Store(int64(len(a.stalenessTrackers)))
	a.fanout.ls.TrackStaleness(a.stalenessTrackers)
	var multiErr error
	for _, x := range a.children {
		err := x.Rollback()
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

func (a *appender) recordLatency() {
	if a.start.IsZero() {
		return
	}
	duration := time.Since(a.start)
	a.fanout.writeLatency.Observe(duration.Seconds())
}

// AppendExemplar satisfies the Appender interface.
func (a *appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendExemplar(ref, l, e)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

// UpdateMetadata satisfies the Appender interface.
func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}

	var multiErr error
	for _, x := range a.children {
		_, err := x.UpdateMetadata(ref, l, m)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}

	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendHistogram(ref, l, t, h, fh)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

func (a *appender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendSTZeroSample(ref, l, t, st)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

func (a *appender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendHistogramSTZeroSample(ref, l, t, st, h, fh)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}
