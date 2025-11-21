package prometheus

import (
	"context"
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

	"github.com/grafana/alloy/internal/service/labelstore"
)

var _ storage.Appendable = (*Fanout)(nil)

// Fanout supports the default Alloy style of appendables since it can go to multiple outputs. It also allows the intercepting of appends.
type Fanout struct {
	mut sync.RWMutex
	// children is where to fan out.
	children       []storage.Appendable
	writeLatency   prometheus.Histogram
	samplesCounter prometheus.Counter
	ls             labelstore.LabelStore

	// lastSeriesCount stores the number of series that were sent through the last appender. It helps to estimate how
	// much memory to allocate for the staleness trackers.
	lastSeriesCount atomic.Int64
}

// NewFanout creates a fanout appendable.
func NewFanout(children []storage.Appendable, register prometheus.Registerer, ls labelstore.LabelStore) *Fanout {
	wl := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "prometheus_fanout_latency",
		Help:    "Write latency for sending to direct and indirect components",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
	})
	_ = register.Register(wl)

	s := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_forwarded_samples_total",
		Help: "Total number of samples sent to downstream components.",
	})
	_ = register.Register(s)

	return &Fanout{
		children:       children,
		writeLatency:   wl,
		samplesCounter: s,
		ls:             ls,
	}
}

// UpdateChildren allows changing of the children of the fanout.
func (f *Fanout) UpdateChildren(children []storage.Appendable) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.children = children
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

	app := &appender{
		children:          make([]storage.Appender, 0),
		fanout:            f,
		stalenessTrackers: make([]labelstore.StalenessTracker, 0, f.lastSeriesCount.Load()),
	}

	for _, x := range f.children {
		if x == nil {
			continue
		}
		app.children = append(app.children, x.Appender(ctx))
	}
	return app
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
	// TODO histograms are not currently tracked for staleness causing them to be held forever
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendHistogram(ref, l, t, h, fh)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

func (a *appender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendCTZeroSample(ref, l, t, ct)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}

func (a *appender) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if a.start.IsZero() {
		a.start = time.Now()
	}
	if ref == 0 {
		ref = storage.SeriesRef(a.fanout.ls.GetOrAddGlobalRefID(l))
	}
	var multiErr error
	for _, x := range a.children {
		_, err := x.AppendHistogramCTZeroSample(ref, l, t, ct, h, fh)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return ref, multiErr
}
