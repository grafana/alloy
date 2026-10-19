package simulation

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "lokisim.sink",
		Stability: featuregate.StabilityExperimental,
		Args:      SinkArgs{},
		Exports:   ReceiverExports{},
		Build: func(o component.Options, args component.Arguments) (component.Component, error) {
			return newSink(o, args.(SinkArgs)), nil
		},
	})
	component.Register(component.Registration{
		Name:      "lokisim.relay",
		Stability: featuregate.StabilityExperimental,
		Args:      RelayArgs{},
		Exports:   ReceiverExports{},
		Build: func(o component.Options, args component.Arguments) (component.Component, error) {
			return newRelay(o, args.(RelayArgs)), nil
		},
	})
	component.Register(component.Registration{
		Name:      "lokisim.source",
		Stability: featuregate.StabilityExperimental,
		Args:      SourceArgs{},
		Build: func(o component.Options, args component.Arguments) (component.Component, error) {
			return newSource(o, args.(SourceArgs))
		},
	})
}

type ReceiverExports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// sink records every entry it receives. It reads nothing until Run starts,
// like every real downstream component.
type SinkArgs struct {
	Run        string        `alloy:"run,attr"`
	Delay      time.Duration `alloy:"delay,attr,optional"`       // Per entry.
	StartDelay time.Duration `alloy:"start_delay,attr,optional"` // Before reading anything.
}

type sink struct {
	id    string
	rec   *recorder
	recv  loki.LogsReceiver
	args  SinkArgs
	delay atomic.Int64
}

func newSink(o component.Options, args SinkArgs) *sink {
	s := &sink{id: o.ID, rec: recorderFor(args.Run), recv: loki.NewLogsReceiver(), args: args}
	s.delay.Store(int64(args.Delay))
	o.OnStateChange(ReceiverExports{Receiver: s.recv})
	return s
}

func (s *sink) Run(ctx context.Context) error {
	select {
	case <-time.After(s.args.StartDelay):
	case <-ctx.Done():
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-s.recv.Chan():
			s.rec.markDelivered(e, s.id)
			if d := time.Duration(s.delay.Load()); d > 0 {
				time.Sleep(d)
			}
		}
	}
}

func (s *sink) Update(args component.Arguments) error {
	s.delay.Store(int64(args.(SinkArgs).Delay))
	return nil
}

// relay is a processor that follows the contract: Update only switches the
// receivers, and on shutdown it hands off the entry it holds within a
// timeout.
type RelayArgs struct {
	Run       string              `alloy:"run,attr"`
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

type relay struct {
	id     string
	rec    *recorder
	recv   loki.LogsReceiver
	fanout *loki.Fanout
}

func newRelay(o component.Options, args RelayArgs) *relay {
	r := &relay{id: o.ID, rec: recorderFor(args.Run), recv: loki.NewLogsReceiver(), fanout: loki.NewFanout(args.ForwardTo)}
	o.OnStateChange(ReceiverExports{Receiver: r.recv})
	return r
}

func (r *relay) Run(ctx context.Context) error {
	// Keep sending for up to a second after shutdown starts. A cancelled
	// Send may have reached some receivers already, so it must not be
	// retried.
	sendCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopTimer := context.AfterFunc(ctx, func() { time.AfterFunc(time.Second, cancel) })
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-r.recv.Chan():
			// Record the path, so an entry that reaches a sink both directly
			// and through a relay doesn't count as a duplicate. Labels are
			// shared between receivers, so copy them first.
			e.Labels = e.Labels.Clone()
			e.Labels["via"] += model.LabelValue(">" + r.id)
			if err := r.fanout.Send(sendCtx, e); err != nil {
				r.rec.markDropped(e, r.id)
				return nil
			}
		}
	}
}

func (r *relay) Update(args component.Arguments) error {
	r.fanout.UpdateChildren(args.(RelayArgs).ForwardTo)
	return nil
}

// Source modes.
const (
	// modeRunner uses loki.SourceRunner.
	modeRunner = "runner"
	// modeLegacy is how most sources work on main: Update switches the
	// receivers, then stops the old producer and starts the new one.
	modeLegacy = "legacy"
	// modeLegacyDeferred switches the receivers last instead, after the old
	// producer stopped (grafana/alloy#7367).
	modeLegacyDeferred = "legacy_deferred"
)

type SourceArgs struct {
	Run       string              `alloy:"run,attr"`
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
	Mode      string              `alloy:"mode,attr,optional"`

	// Changing Generation needs a producer restart.
	Generation int `alloy:"generation,attr,optional"`
	// Resource is held exclusively by a running producer, like a port.
	Resource string `alloy:"resource,attr,optional"`
	// StartFailures makes the first N starts of this generation fail.
	StartFailures int `alloy:"start_failures,attr,optional"`
	// BlockingSends makes the producer hand off the entries it holds before
	// it stops, however long that takes.
	BlockingSends bool `alloy:"blocking_sends,attr,optional"`
	// BatchSize makes the producer send batches of this size.
	BatchSize    int           `alloy:"batch_size,attr,optional"`
	Interval     time.Duration `alloy:"interval,attr,optional"`
	DrainTimeout time.Duration `alloy:"drain_timeout,attr,optional"`
}

type source struct {
	id   string
	rec  *recorder
	mode string

	runner *loki.SourceRunner[SourceArgs]

	// Legacy modes.
	mut      sync.Mutex
	handler  loki.LogsReceiver
	fanout   *legacyFanout
	producer *producer
	drain    time.Duration
}

func newSource(o component.Options, args SourceArgs) (*source, error) {
	s := &source{id: o.ID, rec: recorderFor(args.Run), mode: args.Mode, drain: args.DrainTimeout}
	if s.mode == "" {
		s.mode = modeRunner
	}
	if s.drain == 0 {
		s.drain = time.Second
	}

	switch s.mode {
	case modeRunner:
		s.runner = loki.NewSourceRunner(loki.SourceRunnerOptions[SourceArgs]{
			Logger: o.Logger,
			Start: func(_ context.Context, cfg SourceArgs) (loki.Producer[SourceArgs], error) {
				p, err := startProducer(s.rec, s.id, cfg, outputs{
					entries: s.runner.Entries().Chan(),
					batches: s.runner.Batches().Chan(),
				})
				if err != nil {
					return loki.Producer[SourceArgs]{}, err
				}
				return loki.Producer[SourceArgs]{Stop: p.Stop, Reconfigure: p.Reconfigure}, nil
			},
			DrainTimeout: s.drain,
			MinBackoff:   10 * time.Millisecond,
			MaxBackoff:   100 * time.Millisecond,
			OnDrop:       func(e loki.Entry) { s.rec.markDropped(e, s.id) },
		})
	case modeLegacy, modeLegacyDeferred:
		s.handler = loki.NewLogsReceiver()
		s.fanout = &legacyFanout{children: args.ForwardTo, abort: s.rec.abort}
	default:
		return nil, fmt.Errorf("unknown mode %q", s.mode)
	}

	if err := s.Update(args); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *source) Update(args component.Arguments) error {
	newArgs := args.(SourceArgs)
	cfg := newArgs
	cfg.ForwardTo = nil

	switch s.mode {
	case modeRunner:
		s.runner.Update(newArgs.ForwardTo, cfg)
		return nil
	case modeLegacy:
		s.mut.Lock()
		defer s.mut.Unlock()
		s.fanout.UpdateChildren(newArgs.ForwardTo)
		return s.restartLocked(cfg)
	default: // modeLegacyDeferred
		s.mut.Lock()
		defer s.mut.Unlock()
		defer s.fanout.UpdateChildren(newArgs.ForwardTo)
		return s.restartLocked(cfg)
	}
}

func (s *source) restartLocked(cfg SourceArgs) error {
	if s.producer != nil {
		s.producer.Stop()
		s.producer = nil
	}
	p, err := startProducer(s.rec, s.id, cfg, outputs{entries: s.handler.Chan()})
	if err != nil {
		return err
	}
	s.producer = p
	return nil
}

func (s *source) Run(ctx context.Context) error {
	if s.mode == modeRunner {
		s.runner.Run(ctx)
		return nil
	}

	// Legacy: forward until shutdown, then drain while the producer stops,
	// like loki.Consume and loki.Drain.
	for {
		select {
		case <-ctx.Done():
			s.legacyShutdown()
			return nil
		case e := <-s.handler.Chan():
			if err := s.fanout.Send(ctx, e); err != nil {
				s.rec.markDropped(e, s.id)
			}
		}
	}
}

func (s *source) legacyShutdown() {
	drainCtx, cancel := context.WithTimeout(context.Background(), s.drain)
	defer cancel()
	stopped := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case e := <-s.handler.Chan():
				if err := s.fanout.Send(drainCtx, e); err != nil {
					s.rec.markDropped(e, s.id)
				}
			case <-stopped:
				return
			}
		}
	})
	s.mut.Lock()
	if s.producer != nil {
		s.producer.Stop()
		s.producer = nil
	}
	s.mut.Unlock()
	close(stopped)
	wg.Wait()
}

func (s *source) CurrentHealth() component.Health {
	if s.runner != nil {
		return s.runner.CurrentHealth()
	}
	return component.Health{Health: component.HealthTypeHealthy}
}

// producer generates numbered entries until it's stopped.
type producer struct {
	rec      *recorder
	source   string
	resource string
	holder   string
	cfg      SourceArgs // Only used by Reconfigure.
	interval atomic.Int64
	stop     chan struct{}
	done     chan struct{}
}

// bufferedEntries is how many entries a producer with BlockingSends hands off
// after it's asked to stop.
const bufferedEntries = 2

// outputs is where a producer sends entries.
type outputs struct {
	entries chan<- loki.Entry
	batches chan<- []loki.Entry // nil if batches aren't supported.
}

func startProducer(rec *recorder, source string, cfg SourceArgs, out outputs) (*producer, error) {
	key := fmt.Sprintf("%s/%d", source, cfg.Generation)
	n := rec.countStart(key)
	if n <= cfg.StartFailures {
		return nil, fmt.Errorf("injected start failure %d of %d", n, cfg.StartFailures)
	}
	p := &producer{
		rec:      rec,
		source:   source,
		resource: cfg.Resource,
		holder:   fmt.Sprintf("%s#%d", key, n),
		cfg:      cfg,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	p.interval.Store(int64(cfg.Interval))
	if p.resource != "" {
		if err := rec.acquire(p.resource, p.holder); err != nil {
			return nil, err
		}
	}
	rec.markRunning(source, p.holder, cfg.Generation)
	go p.run(out, cfg)
	return p, nil
}

func (p *producer) run(out outputs, cfg SourceArgs) {
	defer close(p.done)
	for {
		if !cfg.BlockingSends && p.stopping() {
			return
		}
		if p.rec.paused.Load() {
			select {
			case <-p.stop:
				return
			case <-p.rec.abort:
				return
			case <-time.After(time.Millisecond):
				continue
			}
		}

		if !p.send(out, cfg) {
			return
		}
		if cfg.BlockingSends && p.stopping() {
			// The producer still holds buffered entries that it has to hand
			// off before it can stop, like a consumer that has already read a
			// batch of messages.
			for range bufferedEntries {
				if !p.send(out, cfg) {
					return
				}
			}
			return
		}

		if d := time.Duration(p.interval.Load()); d > 0 {
			select {
			case <-time.After(d):
			case <-p.stop:
				return
			}
		}
	}
}

// send hands off one entry, or a batch if BatchSize is set and supported. It
// returns false if the producer must exit.
func (p *producer) send(out outputs, cfg SourceArgs) bool {
	var stop <-chan struct{}
	if !cfg.BlockingSends {
		stop = p.stop
	}

	if cfg.BatchSize > 0 && out.batches != nil {
		batch := make([]loki.Entry, cfg.BatchSize)
		ids := make([]int64, cfg.BatchSize)
		for i := range batch {
			batch[i], ids[i] = p.rec.newEntry(p.source)
		}
		select {
		case out.batches <- batch:
		case <-stop:
			return false
		case <-p.rec.abort:
			return false
		}
		for _, id := range ids {
			p.rec.markProduced(id, p.source)
		}
		return true
	}

	e, id := p.rec.newEntry(p.source)
	select {
	case out.entries <- e:
	case <-stop:
		return false
	case <-p.rec.abort:
		return false
	}
	p.rec.markProduced(id, p.source)
	return true
}

func (p *producer) stopping() bool {
	select {
	case <-p.stop:
		return true
	case <-p.rec.abort:
		return true
	default:
		return false
	}
}

// Reconfigure applies a change of interval without a restart.
func (p *producer) Reconfigure(cfg SourceArgs) bool {
	same := p.cfg
	same.Interval = cfg.Interval
	if !reflect.DeepEqual(same, cfg) {
		return false
	}
	p.cfg = cfg
	p.interval.Store(int64(cfg.Interval))
	return true
}

func (p *producer) Stop() {
	close(p.stop)
	select {
	case <-p.done:
	case <-p.rec.abort:
	}
	p.rec.markStopped(p.source, p.holder)
	if p.resource != "" {
		p.rec.release(p.resource, p.holder)
	}
}

// legacyFanout is loki.Fanout as it was before UpdateChildren stopped
// blocking: Send holds a read lock for the whole send, UpdateChildren takes
// the write lock.
type legacyFanout struct {
	mut      sync.RWMutex
	children []loki.LogsReceiver
	abort    chan struct{}
}

func (f *legacyFanout) Send(ctx context.Context, e loki.Entry) error {
	f.mut.RLock()
	defer f.mut.RUnlock()
	for _, recv := range f.children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.abort:
			return context.Canceled
		case recv.Chan() <- e:
		}
	}
	return nil
}

func (f *legacyFanout) UpdateChildren(children []loki.LogsReceiver) {
	f.mut.RLock()
	equal := slices.Equal(f.children, children)
	f.mut.RUnlock()
	if equal {
		return
	}
	f.mut.Lock()
	f.children = children
	f.mut.Unlock()
}
