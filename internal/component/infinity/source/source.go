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
	// parse builds a frame from a body. Tests replace it before Run.
	parse parseFunc

	// gen is the current config generation. Update replaces it without
	// waiting for a running poll. Each poll loads it once, at its start.
	gen atomic.Pointer[generation]
	// clusterChanged holds one pending cluster change.
	clusterChanged chan struct{}
	// refreshMu makes each ownership refresh look up and store as one step,
	// so a refresh from an old ring cannot overwrite a newer one. Never
	// hold it with mut or a queryState.mut.
	refreshMu sync.Mutex

	mut         sync.RWMutex
	args        Arguments
	otelMetrics []otelcol.Consumer
	otelLogs    []otelcol.Consumer
	queries     map[string]*queryState
	// workers holds the worker count of each query name. A worker can
	// outlive its queryState, so a query that a reload removes and a later
	// reload adds back must see the workers that still run.
	workers map[string]*atomic.Int32
	runCtx  context.Context
}

type parseFunc func(querySpec, []byte) (*data.Frame, error)

// generation holds the settings of one config load. It never changes, so
// a poll that loads it sees a client and specs from the same load.
type generation struct {
	client *http.Client
	// proxyURL is the configured proxy_url, or empty. Error messages name it.
	proxyURL   string
	timeout    time.Duration
	maxSize    int64
	clustering bool
	specs      map[string]querySpec
}

type queryState struct {
	// workers counts the fetch goroutines of this query that still run.
	// A goroutine that passes the timeout keeps running, so it can be 1
	// when a poll starts. All queryStates with the same name share it.
	workers *atomic.Int32
	// assigned tells if the cluster gives this query to this node.
	assigned atomic.Bool
	kick     chan struct{}

	mut              sync.Mutex // guards the fields below
	tracker          *tracker
	owned            bool
	warnedOverride   bool
	warnedReservedUp bool
	loop             *loop // guarded by Component.mut

	hmut    sync.Mutex
	lastErr error
}

// release gives up a query that the cluster moved to another node. The
// caller holds qs.mut.
func (qs *queryState) release() {
	if qs.owned {
		// The new owner takes over. The old series go stale by lookback.
		qs.tracker.reset()
		qs.owned = false
	}
	// This node does not run the query, so its own health must not
	// reflect a failure from before it lost ownership.
	qs.setResult(nil)
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
	_ cluster.Component         = (*Component)(nil)
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
	// Alloy keeps the registry across a failed build. The fanout registers
	// its metrics and ignores a duplicate, so a retry would export the
	// metrics of this failed instance. Thus all checks run before any
	// metric is registered.
	gen, err := newGeneration(opts.ID, args)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:    opts,
		cluster: clusterData.(cluster.Cluster),
		metrics: newSelfMetrics(opts.Registerer),
		prom:    alloyprom.NewFanout(nil, opts.ID, opts.Registerer, lsData.(labelstore.LabelStore)),
		loki:    loki.NewFanout(nil),
		queries: map[string]*queryState{},
		workers: map[string]*atomic.Int32{},
		parse:   buildFrame,
		// The cluster service calls NotifyClusterChange synchronously, so
		// it sends without blocking into this buffer.
		clusterChanged: make(chan struct{}, 1),
	}
	c.apply(args, gen)
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

	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		c.watchCluster(ctx)
	}()

	<-ctx.Done()
	<-watchDone

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
	gen, err := newGeneration(c.opts.ID, args)
	if err != nil {
		return err
	}
	c.apply(args, gen)
	return nil
}

// newGeneration checks args and builds its generation. It does every step
// of a load that can fail, so apply cannot fail.
func newGeneration(id string, args Arguments) (*generation, error) {
	if err := validateOutputs(args); err != nil {
		return nil, err
	}
	client, err := config.NewClientFromConfig(*args.Client.Convert(), id, config.WithUserAgent(userAgent))
	if err != nil {
		return nil, err
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
	var proxyURL string
	if p := args.Client.ProxyConfig; p != nil && p.ProxyURL.URL != nil {
		proxyURL = p.ProxyURL.String()
	}
	return &generation{
		client:     client,
		proxyURL:   proxyURL,
		timeout:    args.Timeout,
		maxSize:    int64(args.MaxResponseSize),
		clustering: args.Clustering.Enabled,
		specs:      specs,
	}, nil
}

// apply makes args and gen the current config.
func (c *Component) apply(args Arguments, gen *generation) {
	specs := gen.specs
	c.loki.UpdateChildren(args.ForwardTo.Logs)

	c.mut.Lock()
	oldGen := c.gen.Load()
	// Only the outputs from before the reload have the series of a removed
	// query, so its stale markers go there.
	oldOutputs := outputs{
		prom:        c.prom,
		loki:        c.loki,
		otelMetrics: c.otelMetrics,
		otelLogs:    c.otelLogs,
	}
	type removedQuery struct {
		name string
		qs   *queryState
		loop *loop
	}
	var removed []removedQuery
	for name, qs := range c.queries {
		if _, keep := specs[name]; keep {
			continue
		}
		removed = append(removed, removedQuery{name: name, qs: qs, loop: qs.loop})
		qs.loop = nil
		delete(c.queries, name)
	}
	c.mut.Unlock()

	// Stop each loop before its tracker is read, so no poll changes the
	// tracker after the read.
	for _, r := range removed {
		if r.loop != nil {
			r.loop.stop()
		}
	}
	for _, r := range removed {
		r.qs.mut.Lock()
		stale := r.qs.tracker.all()
		r.qs.tracker.reset()
		r.qs.mut.Unlock()
		// The series came from the old config. When another node owns the
		// query now, its series are that node's, so do not mark them.
		if oldGen != nil && oldGen.clustering && !c.owns(r.name) {
			stale = nil
		}
		if len(stale) > 0 {
			if err := oldOutputs.sendMetrics(context.Background(), c.opts.ID, r.name, time.Now(), nil, stale); err != nil {
				c.logSendFailure("failed to send stale markers for a removed query", r.name, err)
			}
		}
		c.metrics.deleteQuery(r.name)
	}

	c.prom.UpdateChildren(args.ForwardTo.Metrics)

	var (
		toStop []*loop
		// toLookUp holds the queries whose owner Update looks up before
		// their loops start.
		toLookUp = map[string]*queryState{}
	)

	c.mut.Lock()
	intervalChanged := c.args.Interval != args.Interval
	c.args = args
	c.gen.Store(gen)
	c.otelMetrics = args.Output.Metrics
	c.otelLogs = args.Output.Logs

	clusteringEnabled := gen.clustering && (oldGen == nil || !oldGen.clustering)
	for name := range specs {
		qs, ok := c.queries[name]
		if !ok {
			w := c.workers[name]
			if w == nil {
				w = atomic.NewInt32(0)
				c.workers[name] = w
			}
			qs = &queryState{workers: w, tracker: newTracker(), kick: make(chan struct{}, 1)}
			c.queries[name] = qs
		}
		switch {
		case !gen.clustering:
			// Enabling clustering later must not see a false gain.
			qs.assigned.Store(true)
		case !ok || clusteringEnabled:
			toLookUp[name] = qs
		}
		// A restart drops a pending jitter, so the query waits for its new
		// random offset.
		if intervalChanged && qs.loop != nil {
			toStop = append(toStop, qs.loop)
			qs.loop = nil
		}
	}
	// The loops of removed queries stopped above, so no new worker can
	// start for a name that is not in c.queries. A count above 0 must stay
	// for when the query comes back.
	for name, w := range c.workers {
		if _, ok := c.queries[name]; !ok && w.Load() == 0 {
			delete(c.workers, name)
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

	// A new query must not poll before its owner is known. New loops keep
	// their random offset, so only a loop that Run already started gets a
	// kick below.
	gained := c.refreshOwnership(toLookUp)

	c.mut.Lock()
	for _, qs := range gained {
		if qs.loop != nil {
			kick(qs)
		}
	}
	if c.runCtx != nil {
		for name, qs := range c.queries {
			if qs.loop == nil {
				qs.loop = c.startLoopLocked(name, qs)
			}
		}
	}
	c.mut.Unlock()
}

// startLoopLocked starts the poll loop of one query. The caller holds c.mut.
func (c *Component) startLoopLocked(name string, qs *queryState) *loop {
	interval := c.args.Interval
	return startLoop(c.runCtx, interval, randomOffset(interval), qs.kick,
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

	if g.clustering && !qs.assigned.Load() {
		qs.release()
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
			c.logSendFailure("failed to send stale markers after a format change", name, err)
		}
	}
	frame, err := fetchFrame(ctx, g, s, qs.workers, c.parse)
	// Load the current generation, because a reload that turns clustering
	// on and keeps the interval does not stop this loop.
	if c.gen.Load().clustering && !qs.assigned.Load() {
		// The node lost the query during the fetch. The new owner writes
		// the same series, so this node must not send the result.
		qs.release()
		return
	}
	err = c.emit(ctx, qs, s, start, frame, err)
	c.metrics.pollDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
	if err != nil {
		if ctx.Err() != nil {
			// The loop is stopping. Do not report a failure.
			return
		}
		pe := asPollError(err)
		err = pe
		c.metrics.pollFailures.WithLabelValues(name, pe.reason).Inc()
		c.opts.Logger.Warn("poll failed", "query", name, "reason", pe.reason, "err", pe.Error())
		c.opts.Logger.Debug("poll failed", "query", name, "reason", pe.reason, "detail", pe.detail())
	}
	qs.setResult(err)
}

// watchCluster refreshes ownership once at start and on each cluster
// change. It wakes the loops of the queries this node gains, so they do not
// wait for their next tick. A query this node loses stops at its next poll.
func (c *Component) watchCluster(ctx context.Context) {
	refresh := func() {
		if !c.gen.Load().clustering {
			return
		}
		c.mut.RLock()
		states := maps.Clone(c.queries)
		c.mut.RUnlock()
		for _, qs := range c.refreshOwnership(states) {
			kick(qs)
		}
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.clusterChanged:
			refresh()
		}
	}
}

// refreshOwnership looks up the owner of each query and returns the queries
// that this node gains. It takes no lock that a poll holds, so a blocked
// poll cannot delay it.
func (c *Component) refreshOwnership(states map[string]*queryState) []*queryState {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	var gained []*queryState
	for name, qs := range states {
		mine := c.owns(name)
		if was := qs.assigned.Swap(mine); mine && !was {
			gained = append(gained, qs)
		}
	}
	return gained
}

// kick wakes the loop of qs without blocking. A kick that is already pending
// covers this one.
func kick(qs *queryState) {
	select {
	case qs.kick <- struct{}{}:
	default:
	}
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
			return newEmitError(err)
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
			c.logSendFailure("failed to send stale markers", s.name, sendErr)
		}
		qs.tracker.replace([]sample{up})
		c.metrics.samplesSent.WithLabelValues(s.name).Inc()
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
		return newEmitError(sendErr)
	}
	return nil
}

// logSendFailure logs a failed send. A receiver can return text from a
// remote server, so only the debug line has the error text.
func (c *Component) logSendFailure(msg, query string, err error) {
	pe := newEmitError(err)
	c.opts.Logger.Warn(msg, "query", query, "err", pe.Error())
	c.opts.Logger.Debug(msg, "query", query, "detail", pe.detail())
}

// fetchFrame gets the body and builds the frame in a worker goroutine. The
// timeout covers the request, the body read and parsing.
//
// The poll stops waiting at the deadline, because two steps take no
// context: infinity-libs parsing, and the OAuth token request of
// prometheus/common. A jq or jsonata expression that never ends keeps
// running in the worker. So a query starts no new worker while its last
// one still runs, and this keeps it to one abandoned worker.
func fetchFrame(ctx context.Context, g *generation, s querySpec, workers *atomic.Int32, parse parseFunc) (*data.Frame, error) {
	// A reload that changes the interval stops the loop and abandons its
	// running worker. So the first poll of the new loop can also get here.
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
		f, err := fetchAndBuild(ctx, g, s, parse)
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

func fetchAndBuild(ctx context.Context, g *generation, s querySpec, parse parseFunc) (*data.Frame, error) {
	body := []byte(s.data)
	if s.source == sourceURL {
		req, err := buildRequest(ctx, s)
		if err != nil {
			return nil, &pollError{reason: reasonRequest, msg: fmt.Sprintf("could not build the request for %s", redactURL(s.url)), err: redactErr(err)}
		}
		body, err = fetch(g.client, req, requestTarget{rawURL: s.url, proxyURL: g.proxyURL, timeout: g.timeout}, g.maxSize)
		if err != nil {
			return nil, err
		}
	}
	return parse(s, body)
}

// NotifyClusterChange implements cluster.Component. It never blocks, because
// the cluster service calls each component in turn.
func (c *Component) NotifyClusterChange() {
	if g := c.gen.Load(); g == nil || !g.clustering {
		return
	}
	select {
	case c.clusterChanged <- struct{}{}:
	default:
	}
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
