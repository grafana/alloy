package pyroscope

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfoclient"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	LabelNameDelta   = "__delta__"
	LabelName        = "__name__"
	LabelServiceName = "service_name"

	HeaderContentType = "Content-Type"
)

var NoopAppendable = AppendableFunc(func(_ context.Context, _ labels.Labels, _ []*RawSample) error { return nil })

type Appendable interface {
	debuginfo.Appender

	Appender() Appender
}

type Appender interface {
	Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error
	AppendIngest(ctx context.Context, profile *IncomingProfile) error
}

// RawProfileSeries contains profiles sharing the same labels.
// Appenders must not mutate the series, samples, or profile bytes.
type RawProfileSeries struct {
	Labels  labels.Labels
	Samples []*RawSample
}

// BatchAppender optionally supports sending multiple series in one request.
type BatchAppender interface {
	AppendBatch(ctx context.Context, series []RawProfileSeries) error
}

// AppendBatch uses batch delivery when supported, falling back to individual appends.
func AppendBatch(ctx context.Context, app Appender, series []RawProfileSeries) error {
	if len(series) == 0 {
		return nil
	}
	if batch, ok := app.(BatchAppender); ok {
		return batch.AppendBatch(ctx, series)
	}
	var multiErr error
	for _, s := range series {
		if err := ctx.Err(); err != nil {
			return multierror.Append(multiErr, err)
		}
		if err := app.Append(ctx, s.Labels, s.Samples); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

type RawSample struct {
	ID string
	// raw_profile is the set of bytes of the pprof profile
	RawProfile []byte
}

type IncomingProfile struct {
	// RawBody is the set of bytes of the pprof profile, as its sent by the client
	RawBody []byte
	// ContentType is the content type of the RawBody. This must be sent on to the endpoints.
	ContentType []string
	URL         *url.URL
	Labels      labels.Labels
}

var _ Appendable = (*Fanout)(nil)

// Fanout supports the default Alloy style of appendables since it can go to multiple outputs. It also allows the intercepting of appends.
type Fanout struct {
	mut sync.RWMutex
	// children is where to fan out.
	children []Appendable
	// ComponentID is what component this belongs to.
	componentID    string
	writeLatency   prometheus.Histogram
	samplesCounter prometheus.Counter
}

func (f *Fanout) DebugInfoClients() []*debuginfoclient.Client {
	f.mut.RLock()
	defer f.mut.RUnlock()
	var clients []*debuginfoclient.Client
	for _, c := range f.children {
		clients = append(clients, c.DebugInfoClients()...)
	}
	return clients
}

func (f *Fanout) Upload(j debuginfo.UploadJob) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	for _, c := range f.children {
		c.Upload(j)
	}
}

// NewFanout creates a fanout appendable.
func NewFanout(children []Appendable, componentID string, register prometheus.Registerer) *Fanout {
	wl := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                            "pyroscope_fanout_latency",
		Help:                            "Write latency for sending to pyroscope profiles",
		Buckets:                         prometheus.DefBuckets,
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: 1 * time.Hour,
	})
	_ = register.Register(wl)

	sc := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pyroscope_forwarded_entries_total",
		Help: "Total number of samples sent to downstream components.",
	})
	_ = register.Register(sc)

	return &Fanout{
		children:       children,
		componentID:    componentID,
		writeLatency:   wl,
		samplesCounter: sc,
	}
}

// UpdateChildren allows changing of the children of the fanout.
func (f *Fanout) UpdateChildren(children []Appendable) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.children = children
}

// Children returns the children of the fanout.
func (f *Fanout) Children() []Appendable {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.children
}

// Appender satisfies the Appendable interface.
func (f *Fanout) Appender() Appender {
	f.mut.RLock()
	defer f.mut.RUnlock()

	app := &appender{
		children:       make([]Appender, 0),
		componentID:    f.componentID,
		writeLatency:   f.writeLatency,
		samplesCounter: f.samplesCounter,
	}
	for _, x := range f.children {
		if x == nil {
			continue
		}
		app.children = append(app.children, x.Appender())
	}
	return app
}

func (f *Fanout) String() string {
	return f.componentID + ".receiver"
}

var _ Appender = (*appender)(nil)

type appender struct {
	children       []Appender
	componentID    string
	writeLatency   prometheus.Histogram
	samplesCounter prometheus.Counter
}

// Append satisfies the Appender interface.
func (a *appender) Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error {
	now := time.Now()
	defer func() {
		a.writeLatency.Observe(time.Since(now).Seconds())
	}()

	var multiErr error
	for _, x := range a.children {
		err := x.Append(ctx, labels, samples)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	if multiErr == nil {
		a.samplesCounter.Add(float64(len(samples)))
	}

	return multiErr
}

// AppendIngest satisfies the AppenderIngest interface.
func (a *appender) AppendIngest(ctx context.Context, profile *IncomingProfile) error {
	now := time.Now()
	defer func() {
		a.writeLatency.Observe(time.Since(now).Seconds())
	}()
	var multiErr error
	for _, x := range a.children {
		// Create a copy for each child
		profileCopy := &IncomingProfile{
			RawBody:     profile.RawBody,     // []byte is immutable, safe to share
			ContentType: profile.ContentType, // []string is immutable, safe to share
			URL:         profile.URL,         // URL is immutable once created
			Labels:      profile.Labels.Copy(),
		}

		err := x.AppendIngest(ctx, profileCopy)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

func (a *appender) AppendBatch(ctx context.Context, series []RawProfileSeries) error {
	if len(series) == 0 {
		return nil
	}
	now := time.Now()
	defer func() { a.writeLatency.Observe(time.Since(now).Seconds()) }()
	var multiErr error
	for _, child := range a.children {
		if err := AppendBatch(ctx, child, series); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	if multiErr == nil {
		var count int
		for _, s := range series {
			count += len(s.Samples)
		}
		a.samplesCounter.Add(float64(count))
	}
	return multiErr
}
