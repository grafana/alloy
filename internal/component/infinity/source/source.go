package source

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/common/config"

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

	mut         sync.RWMutex
	args        Arguments
	client      *http.Client
	otelMetrics []otelcol.Consumer
	otelLogs    []otelcol.Consumer
	queries     map[string]*queryState
	runCtx      context.Context
}

type queryState struct {
	mut            sync.Mutex
	spec           querySpec
	tracker        *tracker
	owned          bool
	warnedOverride bool
	loop           *loop // guarded by Component.mut

	hmut    sync.Mutex
	polled  bool
	lastErr error
}

func (qs *queryState) setResult(err error) {
	qs.hmut.Lock()
	defer qs.hmut.Unlock()
	qs.polled = true
	qs.lastErr = err
}

func (qs *queryState) result() error {
	qs.hmut.Lock()
	defer qs.hmut.Unlock()
	return qs.lastErr
}

// pollConfig is a snapshot of the settings a poll needs.
type pollConfig struct {
	client     *http.Client
	timeout    time.Duration
	maxSize    int64
	clustering bool
	out        outputs
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
	specs := make(map[string]querySpec, len(args.Queries))
	for _, q := range args.Queries {
		specs[q.Name] = newQuerySpec(q)
	}

	c.prom.UpdateChildren(args.ForwardTo.Metrics)
	c.loki.UpdateChildren(args.ForwardTo.Logs)

	type removedQuery struct {
		name string
		qs   *queryState
	}
	// pendingSpec is a query whose spec must be set once c.mut is free. A
	// poll can hold qs.mut for a long time, and c.mut must never wait on it.
	type pendingSpec struct {
		qs   *queryState
		spec querySpec
	}
	var (
		toStop  []*loop
		removed []removedQuery
		pending []pendingSpec
	)

	c.mut.Lock()
	intervalChanged := c.args.Interval != args.Interval
	c.args = args
	c.client = client
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
	for name, spec := range specs {
		qs, ok := c.queries[name]
		if !ok {
			qs = &queryState{tracker: newTracker()}
			c.queries[name] = qs
		}
		pending = append(pending, pendingSpec{qs: qs, spec: spec})
		if intervalChanged && qs.loop != nil {
			toStop = append(toStop, qs.loop)
			qs.loop = nil
		}
	}
	c.mut.Unlock()

	for _, l := range toStop {
		l.stop()
	}

	// Set each query's spec now that c.mut is free. A query whose loop was
	// just stopped above has no poll in flight, so this cannot block for
	// long; a query that kept running can still be mid-poll, but that only
	// blocks this one query's spec update, not c.mut or any other query.
	for _, p := range pending {
		p.qs.mut.Lock()
		p.qs.spec = p.spec
		p.qs.mut.Unlock()
	}

	cfg := c.snapshot()
	for _, r := range removed {
		r.qs.mut.Lock()
		stale := r.qs.tracker.all()
		r.qs.tracker.reset()
		r.qs.mut.Unlock()
		if len(stale) > 0 {
			if err := cfg.out.sendMetrics(context.Background(), c.opts.ID, r.name, time.Now(), nil, stale); err != nil {
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

func (c *Component) snapshot() pollConfig {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return pollConfig{
		client:     c.client,
		timeout:    c.args.Timeout,
		maxSize:    int64(c.args.MaxResponseSize),
		clustering: c.args.Clustering.Enabled,
		out: outputs{
			prom:        c.prom,
			loki:        c.loki,
			otelMetrics: c.otelMetrics,
			otelLogs:    c.otelLogs,
		},
	}
}

func (c *Component) poll(ctx context.Context, name string, qs *queryState) {
	cfg := c.snapshot()

	qs.mut.Lock()
	defer qs.mut.Unlock()

	if cfg.clustering && !c.owns(name) {
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
	err := c.runQuery(ctx, cfg, qs, start)
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

// runQuery runs one poll. The caller holds qs.mut.
func (c *Component) runQuery(ctx context.Context, cfg pollConfig, qs *queryState, now time.Time) error {
	s := qs.spec
	job := c.opts.ID
	frame, err := c.fetchFrame(ctx, cfg, s)

	if s.format == "logs" {
		if err != nil {
			return err
		}
		entries, err := frameToEntries(frame, job, s.name, s.logs, now)
		if err != nil {
			return err
		}
		if err := cfg.out.sendLogs(ctx, job, s.name, now, entries); err != nil {
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
		if sendErr := cfg.out.sendMetrics(ctx, job, s.name, now, []sample{up}, stale); sendErr != nil {
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

	current := append(res.samples, upSample(job, s.name, 1))
	stale := qs.tracker.stale(current)
	sendErr := cfg.out.sendMetrics(ctx, job, s.name, now, current, stale)
	// The source worked, so keep the new state even if a receiver failed.
	qs.tracker.replace(current)
	c.metrics.samplesSent.WithLabelValues(s.name).Add(float64(len(current)))
	if sendErr != nil {
		return newPollError(reasonEmit, sendErr)
	}
	return nil
}

func (c *Component) fetchFrame(ctx context.Context, cfg pollConfig, s querySpec) (*data.Frame, error) {
	body := []byte(s.data)
	if s.source == "url" {
		rctx, cancel := context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
		req, err := buildRequest(rctx, s)
		if err != nil {
			return nil, newPollError(reasonRequest, fmt.Errorf("building the request for %s: %w", redactURL(s.url), err))
		}
		body, err = fetch(cfg.client, req, cfg.maxSize)
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
