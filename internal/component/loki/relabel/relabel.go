package relabel

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.relabel",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.relabel
// component.
type Arguments struct {
	// Where the relabeled metrics should be forwarded to.
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	// The relabelling rules to apply to each log entry before it's forwarded.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`

	// The maximum number of items to hold in the component's LRU cache.
	MaxCacheSize int `alloy:"max_cache_size,attr,optional"`

	// MaxForwardQueueSize controls the maximum number of log entries buffered
	// per downstream component. This prevents a slow destination from blocking
	// other destinations. Default is 100000.
	MaxForwardQueueSize int `alloy:"max_forward_queue_size,attr,optional"`

	// BlockOnFull controls behavior when a destination queue is full.
	// If false (default), log entries are dropped when the queue is full.
	// If true, the component will retry with exponential backoff, which may
	// slow down the entire pipeline but prevents data loss.
	BlockOnFull bool `alloy:"block_on_full,attr,optional"`
}

// DefaultArguments provides the default arguments for the loki.relabel
// component.
var DefaultArguments = Arguments{
	MaxCacheSize:        10_000,
	MaxForwardQueueSize: 100_000,
	BlockOnFull:         false,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports holds values which are exported by the loki.relabel component.
type Exports struct {
	Receiver loki.LogsReceiver   `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the loki.relabel component.
type Component struct {
	opts    component.Options
	metrics *metrics

	mut                 sync.RWMutex
	rcs                 []*relabel.Config
	receiver            loki.LogsReceiver
	fanout              []loki.LogsReceiver
	queues              []*destinationQueue
	maxForwardQueueSize int
	blockOnFull         bool

	cache        *lru.Cache
	maxCacheSize int

	debugDataPublisher livedebugging.DebugDataPublisher

	builder labels.ScratchBuilder
}

// Backoff constants for blocking mode, similar to Prometheus remote write.
const (
	minBackoff = 5 * time.Millisecond
	maxBackoff = 5 * time.Second
)

// destinationQueue manages a buffered queue for a single destination to ensure
// FIFO ordering while preventing a slow destination from blocking others.
type destinationQueue struct {
	receiver loki.LogsReceiver
	buffer   chan loki.Entry
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func newDestinationQueue(receiver loki.LogsReceiver, size int) *destinationQueue {
	dq := &destinationQueue{
		receiver: receiver,
		buffer:   make(chan loki.Entry, size),
		stopCh:   make(chan struct{}),
	}
	dq.wg.Add(1)
	go dq.run()
	return dq
}

func (dq *destinationQueue) run() {
	defer dq.wg.Done()
	for {
		select {
		case <-dq.stopCh:
			return
		case entry := <-dq.buffer:
			select {
			case <-dq.stopCh:
				return
			case dq.receiver.Chan() <- entry:
			}
		}
	}
}

// send attempts to queue an entry for sending without blocking.
// Returns true if queued, false if buffer is full.
func (dq *destinationQueue) send(entry loki.Entry) bool {
	select {
	case dq.buffer <- entry:
		return true
	default:
		return false
	}
}

// sendWithBackoff attempts to queue an entry, retrying with exponential backoff
// if the buffer is full. Returns true if queued, false if stopped during retry.
// The metrics parameter is used to track retry attempts.
func (dq *destinationQueue) sendWithBackoff(entry loki.Entry, m *metrics) bool {
	// First try without blocking
	select {
	case dq.buffer <- entry:
		return true
	default:
	}

	// Buffer is full, retry with backoff
	backoff := minBackoff
	for {
		select {
		case <-dq.stopCh:
			return false
		default:
		}

		m.enqueueRetriesTotal.Inc()

		select {
		case <-dq.stopCh:
			return false
		case <-time.After(backoff):
		}

		select {
		case dq.buffer <- entry:
			return true
		default:
			// Still full, increase backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (dq *destinationQueue) stop() {
	close(dq.stopCh)
	dq.wg.Wait()
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new loki.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New(args.MaxCacheSize)
	if err != nil {
		return nil, err
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		metrics:            newMetrics(o.Registerer),
		cache:              cache,
		maxCacheSize:       args.MaxCacheSize,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
		builder:            labels.NewScratchBuilder(0),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
	o.OnStateChange(Exports{Receiver: c.receiver, Rules: args.RelabelConfigs})

	// Call to Update() to set the relabelling rules once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		// Stop all destination queues
		c.mut.Lock()
		for _, q := range c.queues {
			q.stop()
		}
		c.queues = nil
		c.mut.Unlock()
	}()

	componentID := livedebugging.ComponentID(c.opts.ID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.metrics.entriesProcessed.Inc()
			lbls := c.relabel(entry)

			count := uint64(1)
			if len(lbls) == 0 {
				count = 0 // if no labels are left, the count is not incremented because the log will be filtered out
			}
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.LokiLog,
				count,
				func() string {
					return fmt.Sprintf("entry: %s, labels: %s => %s", entry.Line, entry.Labels.String(), lbls.String())
				},
			))

			if len(lbls) == 0 {
				level.Debug(c.opts.Logger).Log("msg", "dropping entry after relabeling", "labels", entry.Labels.String())
				continue
			}

			c.metrics.entriesOutgoing.Inc()
			entry.Labels = lbls

			// Send to each destination's queue. Each destination has its own
			// buffered queue with a dedicated worker goroutine, ensuring FIFO
			// ordering while preventing a slow destination from blocking others.
			// See https://github.com/grafana/alloy/issues/2194
			c.mut.RLock()
			queues := c.queues
			blockOnFull := c.blockOnFull
			c.mut.RUnlock()
			for _, q := range queues {
				var sent bool
				if blockOnFull {
					sent = q.sendWithBackoff(entry, c.metrics)
				} else {
					sent = q.send(entry)
				}
				if !sent {
					c.metrics.droppedEntriesTotal.Inc()
					level.Warn(c.opts.Logger).Log("msg", "dropping log entry because destination queue is full", "labels", entry.Labels.String())
				}
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	newRCS := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)

	// Update fanout and queues. Each destination gets its own queue to ensure
	// FIFO ordering while preventing a slow destination from blocking others.
	// See https://github.com/grafana/alloy/issues/2194
	queueSize := newArgs.MaxForwardQueueSize
	if queueSize <= 0 {
		queueSize = DefaultArguments.MaxForwardQueueSize
	}
	c.mut.Lock()
	oldQueues := c.queues
	if relabelingChanged(c.rcs, newRCS) {
		level.Debug(c.opts.Logger).Log("msg", "received new relabel configs, purging cache")
		c.cache.Purge()
		c.metrics.cacheSize.Set(0)
	}
	if newArgs.MaxCacheSize != c.maxCacheSize {
		evicted := c.cache.Resize(newArgs.MaxCacheSize)
		if evicted > 0 {
			level.Debug(c.opts.Logger).Log("msg", "resizing the cache lead to evicting of items", "len_items_evicted", evicted)
		}
	}
	c.rcs = newRCS
	c.fanout = newArgs.ForwardTo
	c.maxForwardQueueSize = queueSize
	c.blockOnFull = newArgs.BlockOnFull
	c.queues = make([]*destinationQueue, len(newArgs.ForwardTo))
	for i, receiver := range newArgs.ForwardTo {
		c.queues[i] = newDestinationQueue(receiver, queueSize)
	}
	c.mut.Unlock()

	// Stop old queues after releasing the lock to avoid blocking
	for _, q := range oldQueues {
		q.stop()
	}

	c.opts.OnStateChange(Exports{Receiver: c.receiver, Rules: newArgs.RelabelConfigs})

	return nil
}

func relabelingChanged(prev, next []*relabel.Config) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}

type cacheItem struct {
	original  model.LabelSet
	relabeled model.LabelSet
}

// TODO(@tpaschalis) It's unfortunate how we have to cast back and forth
// between model.LabelSet (map) and labels.Labels (slice). Promtail does
// not have this issue as relabel config rules are only applied to targets.
// Do we want to use labels.Labels in loki.Entry instead?
func (c *Component) relabel(e loki.Entry) model.LabelSet {
	hash := e.Labels.Fingerprint()

	// Let's look in the cache for the hash of the entry's labels.
	val, found := c.cache.Get(hash)

	// We've seen this hash before; let's see if we've already relabeled this
	// specific entry before and can return early, or if it's a collision.
	if found {
		for _, ci := range val.([]cacheItem) {
			if e.Labels.Equal(ci.original) {
				c.metrics.cacheHits.Inc()
				return ci.relabeled
			}
		}
	}

	// Seems like it's either a new entry or a hash collision.
	c.metrics.cacheMisses.Inc()
	relabeled := c.process(e)

	// In case it's a new hash, initialize it as a new cacheItem.
	// If it was a collision, append the result to the cached slice.
	if !found {
		val = []cacheItem{{e.Labels, relabeled}}
	} else {
		val = append(val.([]cacheItem), cacheItem{e.Labels, relabeled})
	}

	c.cache.Add(hash, val)
	c.metrics.cacheSize.Set(float64(c.cache.Len()))

	return relabeled
}

func (c *Component) process(e loki.Entry) model.LabelSet {
	c.builder.Reset()
	for k, v := range e.Labels {
		c.builder.Add(string(k), string(v))
	}
	c.builder.Sort()
	lbls := c.builder.Labels()
	lbls, _ = relabel.Process(lbls, c.rcs...)

	relabeled := make(model.LabelSet, lbls.Len())
	lbls.Range(func(lbl labels.Label) {
		relabeled[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})
	return relabeled
}

func (c *Component) LiveDebugging() {}
