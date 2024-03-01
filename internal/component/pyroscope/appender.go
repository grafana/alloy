package pyroscope

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	LabelNameDelta = "__delta__"
)

var NoopAppendable = AppendableFunc(func(_ context.Context, _ labels.Labels, _ []*RawSample) error { return nil })

type Appendable interface {
	Appender() Appender
}

type Appender interface {
	Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error
}

type RawSample struct {
	// raw_profile is the set of bytes of the pprof profile
	RawProfile []byte
}

var _ Appendable = (*Fanout)(nil)

// Fanout supports the default Flow style of appendables since it can go to multiple outputs. It also allows the intercepting of appends.
type Fanout struct {
	mut sync.RWMutex
	// children is where to fan out.
	children []Appendable
	// ComponentID is what component this belongs to.
	componentID  string
	writeLatency prometheus.Histogram
}

// NewFanout creates a fanout appendable.
func NewFanout(children []Appendable, componentID string, register prometheus.Registerer) *Fanout {
	wl := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "pyroscope_fanout_latency",
		Help: "Write latency for sending to pyroscope profiles",
	})
	_ = register.Register(wl)
	return &Fanout{
		children:     children,
		componentID:  componentID,
		writeLatency: wl,
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
		children:     make([]Appender, 0),
		componentID:  f.componentID,
		writeLatency: f.writeLatency,
	}
	for _, x := range f.children {
		if x == nil {
			continue
		}
		app.children = append(app.children, x.Appender())
	}
	return app
}

var _ Appender = (*appender)(nil)

type appender struct {
	children     []Appender
	componentID  string
	writeLatency prometheus.Histogram
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
	return multiErr
}

type AppendableFunc func(ctx context.Context, labels labels.Labels, samples []*RawSample) error

func (f AppendableFunc) Append(ctx context.Context, labels labels.Labels, samples []*RawSample) error {
	return f(ctx, labels, samples)
}

func (f AppendableFunc) Appender() Appender {
	return f
}
