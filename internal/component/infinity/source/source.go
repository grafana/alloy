package source

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/common/config"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/useragent"
)

// userAgent is the User-Agent this component sends on every HTTP request.
var userAgent = useragent.Get()

func init() {
	component.Register(component.Registration{
		Name:      "infinity.source",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Component implements infinity.source.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster
	metrics *selfMetrics
	prom    *alloyprom.Fanout
	loki    *loki.Fanout

	// gen is the current config generation. Update replaces it without
	// waiting for a running poll. Each poll loads it once, at its start.
	gen atomic.Pointer[generation]

	mut         sync.RWMutex
	args        Arguments
	otelMetrics []otelcol.Consumer
	otelLogs    []otelcol.Consumer
	queries     map[string]*queryState
	runCtx      context.Context
}

// generation holds the settings of one config load. It never changes, so
// a poll that loads it sees a client and specs from the same load.
type generation struct {
	client     *http.Client
	timeout    time.Duration
	maxSize    int64
	clustering bool
	specs      map[string]querySpec
}

type queryState struct {
	// workers counts the fetch goroutines of this query that still run.
	// A goroutine that passes the timeout keeps running, so it can be 1
	// when a poll starts.
	workers atomic.Int32

	mut              sync.Mutex // guards the fields below
	tracker          *tracker
	owned            bool
	warnedOverride   bool
	warnedReservedUp bool
	loop             *loop // guarded by Component.mut

	hmut    sync.Mutex
	lastErr error
}

func (qs *queryState) setResult(err error) {
	qs.hmut.Lock()
	defer qs.hmut.Unlock()
	qs.lastErr = err
}

func (qs *queryState) result() error {
	qs.hmut.Lock()
	defer qs.hmut.Unlock()
	return qs.lastErr
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates an infinity.source component.
func New(opts component.Options, args Arguments) (*Component, error) {
	clusterData, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, err
	}
	lsData, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	m, err := newSelfMetrics(opts.Registerer)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:    opts,
		cluster: clusterData.(cluster.Cluster),
		metrics: m,
		prom:    alloyprom.NewFanout(nil, opts.ID, opts.Registerer, lsData.(labelstore.LabelStore)),
		loki:    loki.NewFanout(nil),
		queries: map[string]*queryState{},
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	c.mut.Lock()
	c.runCtx = ctx
	for name, qs := range c.queries {
		qs.loop = c.startLoopLocked(name, qs)
	}
	c.mut.Unlock()

	<-ctx.Done()

	c.mut.Lock()
	c.runCtx = nil
	var loops []*loop
	for _, qs := range c.queries {
		if qs.loop != nil {
			loops = append(loops, qs.loop)
			qs.loop = nil
		}
	}
	c.mut.Unlock()
	// Shutdown sends no stale markers, the same as prometheus.scrape.
	for _, l := range loops {
		l.stop()
	}
	return nil
}

// Update implements component.Component.
func (c *Component) Update(newConfig component.Arguments) error {
	args := newConfig.(Arguments)
	if err := validateOutputs(args); err != nil {
		return err
	}
	client, err := config.NewClientFromConfig(*args.Client.Convert(), c.opts.ID, config.WithUserAgent(userAgent))
	if err != nil {
		return err
	}
	// The client adds its headers to each request, so a default Accept
	// would be sent next to the client's own.
	clientAccept := args.Client.HTTPHeaders != nil && hasHeader(args.Client.HTTPHeaders.Headers, "Accept")
	specs := make(map[string]querySpec, len(args.Queries))
	for _, q := range args.Queries {
		spec := newQuerySpec(q)
		if clientAccept {
			spec.accept = ""
		}
		specs[q.Name] = spec
	}
	gen := &generation{
		client:     client,
		timeout:    args.Timeout,
		maxSize:    int64(args.MaxResponseSize),
		clustering: args.Clustering.Enabled,
		specs:      specs,
	}

	c.prom.UpdateChildren(args.ForwardTo.Metrics)
	c.loki.UpdateChildren(args.ForwardTo.Logs)

	type removedQuery struct {
		name string
		qs   *queryState
	}
	var (
		toStop  []*loop
		removed []removedQuery
	)

	c.mut.Lock()
	intervalChanged := c.args.Interval != args.Interval
	c.args = args
	oldGen := c.gen.Load()
	c.gen.Store(gen)
	c.otelMetrics = args.Output.Metrics
	c.otelLogs = args.Output.Logs

	for name, qs := range c.queries {
		if _, keep := specs[name]; keep {
			continue
		}
		if qs.loop != nil {
			toStop = append(toStop, qs.loop)
			qs.loop = nil
		}
		removed = append(removed, removedQuery{name: name, qs: qs})
		delete(c.queries, name)
	}
	for name := range specs {
		qs, ok := c.queries[name]
		if !ok {
			qs = &queryState{tracker: newTracker()}
			c.queries[name] = qs
		}
		if intervalChanged && qs.loop != nil {
			toStop = append(toStop, qs.loop)
			qs.loop = nil
		}
	}
	c.mut.Unlock()

	for _, l := range toStop {
		l.stop()
	}
	// Close after the stopped loops end, so their connections are idle. A
	// poll that is still running keeps its connection until the transport's
	// idle timeout closes it.
	if oldGen != nil {
		oldGen.client.CloseIdleConnections()
	}

	for _, r := range removed {
		r.qs.mut.Lock()
		stale := r.qs.tracker.all()
		r.qs.tracker.reset()
		r.qs.mut.Unlock()
		if len(stale) > 0 {
			if err := c.outputs().sendMetrics(context.Background(), c.opts.ID, r.name, time.Now(), nil, stale); err != nil {
				c.opts.Logger.Warn("failed to send stale markers for a removed query", "query", r.name, "err", err)
			}
		}
		c.metrics.deleteQuery(r.name)
	}

	c.mut.Lock()
	if c.runCtx != nil {
		for name, qs := range c.queries {
			if qs.loop == nil {
				qs.loop = c.startLoopLocked(name, qs)
			}
		}
	}
	c.mut.Unlock()
	return nil
}

// startLoopLocked starts the poll loop of one query. The caller holds c.mut.
func (c *Component) startLoopLocked(name string, qs *queryState) *loop {
	interval := c.args.Interval
	return startLoop(c.runCtx, interval, randomOffset(interval),
		func(ctx context.Context) { c.poll(ctx, name, qs) },
		func() { c.metrics.pollsOverrun.WithLabelValues(name).Inc() },
	)
}

// outputs returns the current outputs. A poll calls it just before it
// sends, so a reload during the poll changes where the result goes.
func (c *Component) outputs() outputs {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return outputs{
		prom:        c.prom,
		loki:        c.loki,
		otelMetrics: c.otelMetrics,
		otelLogs:    c.otelLogs,
	}
}

func (c *Component) poll(ctx context.Context, name string, qs *queryState) {
	g := c.gen.Load()
	s, ok := g.specs[name]
	if !ok {
		// A reload removed the query and is stopping this loop.
		return
	}

	qs.mut.Lock()
	defer qs.mut.Unlock()

	if g.clustering && !c.owns(name) {
		if qs.owned {
			// The new owner takes over. The old series go stale by lookback.
			qs.tracker.reset()
			qs.owned = false
		}
		// This node does not run the query, so its own health must not
		// reflect a failure from before it lost ownership.
		qs.setResult(nil)
		return
	}
	qs.owned = true

	start := time.Now()
	if s.format == formatLogs && qs.tracker.len() > 0 && ctx.Err() == nil {
		// The query was a table query before a reload. Update does not wait
		// for qs.mut, so the poll ends the old series here.
		stale := qs.tracker.all()
		qs.tracker.reset()
		if err := c.outputs().sendMetrics(ctx, c.opts.ID, name, start, nil, stale); err != nil {
			c.metrics.pollFailures.WithLabelValues(name, reasonEmit).Inc()
			c.opts.Logger.Warn("failed to send stale markers after a format change", "query", name, "err", err)
		}
	}
	frame, err := fetchFrame(ctx, g, s, &qs.workers)
	err = c.emit(ctx, qs, s, start, frame, err)
	c.metrics.pollDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
	if err != nil {
		if ctx.Err() != nil {
			// The loop is stopping. Do not report a failure.
			return
		}
		c.metrics.pollFailures.WithLabelValues(name, reasonOf(err)).Inc()
		c.opts.Logger.Warn("poll failed", "query", name, "reason", reasonOf(err), "err", err)
	}
	qs.setResult(err)
}

func (c *Component) owns(name string) bool {
	if !c.cluster.Ready() {
		return false
	}
	peers, err := c.cluster.Lookup(shard.StringKey(c.opts.ID+"/"+name), 1, shard.OpReadWrite)
	if err != nil || len(peers) == 0 {
		return true
	}
	return peers[0].Self
}

// emit maps the result of a fetch and sends it. fetchErr is the fetch
// error, if any. The caller holds qs.mut.
func (c *Component) emit(ctx context.Context, qs *queryState, s querySpec, now time.Time, frame *data.Frame, fetchErr error) error {
	job := c.opts.ID
	err := fetchErr

	if s.format == formatLogs {
		if err != nil {
			return err
		}
		entries, err := frameToEntries(frame, job, s.name, s.logs, now)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			// The loop is stopping or reloading. Drop the result.
			return err
		}
		if err := c.outputs().sendLogs(ctx, job, s.name, now, entries); err != nil {
			return newPollError(reasonEmit, err)
		}
		c.metrics.entriesSent.WithLabelValues(s.name).Add(float64(len(entries)))
		return nil
	}

	var res metricsResult
	if err == nil {
		res, err = frameToSamples(frame, job, s.name, s.metrics)
	}
	if err != nil {
		if ctx.Err() != nil {
			// The loop is stopping or reloading. Do not fake an outage.
			return err
		}
		up := upSample(job, s.name, 0)
		stale := qs.tracker.allExcept(up.labels)
		if sendErr := c.outputs().sendMetrics(ctx, job, s.name, now, []sample{up}, stale); sendErr != nil {
			c.metrics.pollFailures.WithLabelValues(s.name, reasonEmit).Inc()
			c.opts.Logger.Warn("failed to send stale markers", "query", s.name, "err", sendErr)
		}
		qs.tracker.replace([]sample{up})
		return err
	}

	if res.duplicates > 0 {
		c.metrics.duplicateSeries.WithLabelValues(s.name).Add(float64(res.duplicates))
		c.opts.Logger.Warn("query produced rows with the same labels, kept the first of each", "query", s.name, "duplicates", res.duplicates)
	}
	if res.overridden && !qs.warnedOverride {
		qs.warnedOverride = true
		c.opts.Logger.Warn("a column named job or instance was replaced by the component label", "query", s.name)
	}
	if res.reservedUp && !qs.warnedReservedUp {
		qs.warnedReservedUp = true
		c.opts.Logger.Warn("a column whose metric name is up was dropped because up is reserved for the synthetic sample", "query", s.name)
	}

	if err := ctx.Err(); err != nil {
		// The loop is stopping or reloading. Drop the result and keep the
		// tracker as it was.
		return err
	}
	current := append(res.samples, upSample(job, s.name, 1))
	stale := qs.tracker.stale(current)
	sendErr := c.outputs().sendMetrics(ctx, job, s.name, now, current, stale)
	// The source worked, so keep the new state even if a receiver failed.
	qs.tracker.replace(current)
	c.metrics.samplesSent.WithLabelValues(s.name).Add(float64(len(current)))
	if sendErr != nil {
		return newPollError(reasonEmit, sendErr)
	}
	return nil
}

// fetchFrame gets the body and builds the frame in a worker goroutine. The
// timeout covers the request, the body read and parsing.
//
// The poll stops waiting at the deadline, because two steps take no
// context: infinity-libs parsing, and the OAuth token request of
// prometheus/common. A jq or jsonata expression that never ends keeps
// running in the worker. So a query starts no new worker while its last
// one still runs, and this keeps it to one abandoned worker.
func fetchFrame(ctx context.Context, g *generation, s querySpec, workers *atomic.Int32) (*data.Frame, error) {
	if workers.Load() > 0 {
		return nil, newPollError(reasonTimeout, errors.New("the previous poll is still running"))
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	type result struct {
		frame *data.Frame
		err   error
	}
	// The buffer lets an abandoned worker send and exit.
	done := make(chan result, 1)
	workers.Inc()
	go func() {
		f, err := fetchAndBuild(ctx, g, s)
		workers.Dec()
		done <- result{frame: f, err: err}
	}()
	select {
	case r := <-done:
		return r.frame, r.err
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, newPollError(reasonTimeout, errors.New("the request or parsing took longer than timeout"))
		}
		return nil, ctx.Err()
	}
}

func fetchAndBuild(ctx context.Context, g *generation, s querySpec) (*data.Frame, error) {
	body := []byte(s.data)
	if s.source == sourceURL {
		req, err := buildRequest(ctx, s)
		if err != nil {
			return nil, newPollError(reasonRequest, fmt.Errorf("building the request for %s: %w", redactURL(s.url), redactErr(err)))
		}
		body, err = fetch(g.client, req, g.maxSize)
		if err != nil {
			return nil, err
		}
	}
	return buildFrame(s, body)
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.mut.RLock()
	names := slices.Sorted(maps.Keys(c.queries))
	states := make([]*queryState, len(names))
	for i, n := range names {
		states[i] = c.queries[n]
	}
	c.mut.RUnlock()

	var (
		failed   []string
		firstErr error
	)
	for i, qs := range states {
		if err := qs.result(); err != nil {
			failed = append(failed, names[i])
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if len(failed) == 0 {
		return component.Health{Health: component.HealthTypeHealthy, Message: "all queries are healthy", UpdateTime: time.Now()}
	}
	msg := fmt.Sprintf("query %q failed: %s", failed[0], firstErr)
	switch n := len(failed) - 1; {
	case n == 1:
		msg += " (and 1 other query)"
	case n > 1:
		msg += fmt.Sprintf(" (and %d other queries)", n)
	}
	return component.Health{Health: component.HealthTypeUnhealthy, Message: msg, UpdateTime: time.Now()}
}
