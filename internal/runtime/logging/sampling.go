package logging

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	slogsampling "github.com/samber/slog-sampling"
	"github.com/samber/slog-sampling/buffer"
)

type componentInfo struct{ id, path string }

// sniffComponent inspects the attrs of a log record for component/controller
// identifying attributes and merges them onto base, returning the result.
// component_id wins over controller_id; component_path is captured verbatim.
func sniffComponent(base componentInfo, attrs []slog.Attr) componentInfo {
	c := base
	for _, a := range attrs {
		switch a.Key {
		case "component_id":
			c.id = a.Value.String()
		case "controller_id":
			if c.id == "" {
				c.id = a.Value.String()
			}
		case "component_path":
			c.path = a.Value.String()
		}
	}
	return c
}

type ctxKey struct{}

// withComponent stores componentInfo on ctx so that a downstream Matcher can
// read it back without threading it through every log call site.
func withComponent(ctx context.Context, c componentInfo) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// componentFromCtx returns the componentInfo previously stored by
// withComponent, or the zero value if none was stored.
func componentFromCtx(ctx context.Context) componentInfo {
	c, _ := ctx.Value(ctxKey{}).(componentInfo)
	return c
}

// compMatcher is the slog-sampling Matcher used to key the rate limiter's
// per-signature counters. Two records are considered the same "signature"
// (and thus share a rate-limit budget) when they share the same component
// path, component id, level, and message.
//
// component_path alone is not enough: it identifies the parent/module path
// (e.g. "/" for every top-level component), so distinct top-level components
// emitting the same message at the same level would otherwise collapse onto
// one signature and cross-suppress each other. component_id disambiguates
// them.
func compMatcher(ctx context.Context, r *slog.Record) string {
	c := componentFromCtx(ctx)
	return c.path + "\x00" + c.id + "\x00" + r.Level.String() + "\x00" + r.Message
}

// levelString maps a slog.Level to the lowercase level label used on the
// suppressed-lines metric.
func levelString(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "debug"
	case l < slog.LevelWarn:
		return "info"
	case l < slog.LevelError:
		return "warn"
	default:
		return "error"
	}
}

// rateLimitMetrics tracks how many log lines the rate limiter has dropped,
// broken down by level and component. A nil *rateLimitMetrics is valid and
// its methods are no-ops, so callers need not special-case a missing
// registerer.
type rateLimitMetrics struct {
	suppressed *prometheus.CounterVec
}

// newRateLimitMetrics registers the alloy_logging_suppressed_lines_total
// counter vector against reg. It returns nil if reg is nil, so that callers
// who don't want metrics (e.g. tests) can pass a nil registerer safely.
func newRateLimitMetrics(reg prometheus.Registerer) *rateLimitMetrics {
	if reg == nil {
		return nil
	}
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "alloy_logging_suppressed_lines_total",
		Help: "Total log lines dropped by the logger's rate limiter, by level and component.",
	}, []string{"level", "component_id"})
	if existing := mustRegisterOrReturnExisting(reg, cv); existing != nil {
		cvExisting, ok := existing.(*prometheus.CounterVec)
		if !ok {
			return nil
		}
		cv = cvExisting
	}
	return &rateLimitMetrics{suppressed: cv}
}

// onDropped is the slog-sampling OnDropped hook: it increments the
// suppressed-lines counter for the record's level and component.
func (m *rateLimitMetrics) onDropped(ctx context.Context, r slog.Record) {
	if m == nil {
		return
	}
	m.suppressed.WithLabelValues(levelString(r.Level), componentFromCtx(ctx).id).Inc()
}

// mustRegisterOrReturnExisting registers c against reg. If c is already
// registered (e.g. because multiple Logger instances share a registerer),
// it returns the previously registered collector instead of panicking.
// This is a local copy rather than a dependency on internal/util, which
// would create an import cycle.
func mustRegisterOrReturnExisting(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		panic(err)
	}
	return nil
}

// buildRoot returns the sampling-wrapped terminal handler when rate limiting
// is enabled, else it returns terminal unchanged.
func buildRoot(o RateLimitingOptions, terminal slog.Handler, m *rateLimitMetrics) slog.Handler {
	if !o.Enabled {
		return terminal
	}
	opt := slogsampling.ThresholdSamplingOption{
		Tick:                o.Tick,
		Threshold:           o.Threshold,
		Rate:                o.Rate,
		Matcher:             compMatcher,
		Buffer:              buffer.NewLRUBuffer[string](o.MaxSignatures),
		OnDropped:           m.onDropped,
		IncludeDroppedCount: true,
	}
	return opt.NewMiddleware()(terminal)
}

// replayOp captures a single WithAttrs or WithGroup call so that it can be
// replayed onto a freshly (re-)derived terminal handler. Exactly one of
// attrs or group is set.
type replayOp struct {
	attrs []slog.Attr
	group string
}

// versionedHandler pairs the current sampling-wrapped root handler with a
// version number that's bumped whenever rate-limiting configuration changes
// (see Task 4). samplingInjector uses the version to know when its cached
// derived handler is stale and must be re-derived.
type versionedHandler struct {
	version uint64
	h       slog.Handler
}

// cachedHandler is a samplingInjector's memoized replay of its ops onto a
// particular version of the root handler.
type cachedHandler struct {
	version uint64
	h       slog.Handler
}

// samplingInjector is a slog.Handler that sits between component loggers and
// the shared, possibly rate-limited, root handler. It:
//
//   - Tracks component identity (comp) sniffed from WithAttrs calls, so the
//     rate limiter's Matcher can key on component path via the context.
//   - Records every WithAttrs/WithGroup call as a replayOp, so that when the
//     root handler is swapped out (e.g. rate-limiting config changes) or
//     bypassed (empty-message records), those calls can be replayed onto the
//     new/bare terminal handler to reproduce the same rendering.
//   - Bypasses the rate limiter entirely for empty-message records, which
//     would otherwise all collapse onto the same signature.
type samplingInjector struct {
	comp componentInfo
	ops  []replayOp

	holder *atomic.Pointer[versionedHandler]
	bare   slog.Handler // root terminal (no per-component attrs), for empty-message bypass

	cache     atomic.Pointer[cachedHandler]
	bareCache atomic.Pointer[slog.Handler]
}

// newSamplingInjector creates a samplingInjector rooted at holder (the
// current, possibly sampling-wrapped, root handler) with bare as the
// terminal handler used to bypass sampling for empty-message records.
func newSamplingInjector(holder *atomic.Pointer[versionedHandler], bare slog.Handler) *samplingInjector {
	return &samplingInjector{holder: holder, bare: bare}
}

// replay re-applies a recorded sequence of WithAttrs/WithGroup calls onto h,
// in order, reproducing the derived handler that would have resulted from
// making those calls directly against h.
func replay(h slog.Handler, ops []replayOp) slog.Handler {
	for _, op := range ops {
		if op.group != "" {
			h = h.WithGroup(op.group)
		} else {
			h = h.WithAttrs(op.attrs)
		}
	}
	return h
}

// clone returns a new samplingInjector sharing this injector's holder and
// bare terminal, with an independent copy of ops and freshly reset caches
// (since a fresh ops slice means any previously cached replay is stale).
func (s *samplingInjector) clone() *samplingInjector {
	// New injector shares holder/bare; caches reset (ops differ).
	ns := &samplingInjector{comp: s.comp, holder: s.holder, bare: s.bare}
	ns.ops = make([]replayOp, len(s.ops), len(s.ops)+1)
	copy(ns.ops, s.ops)
	return ns
}

// WithAttrs returns a new handler with attrs bound. It also sniffs attrs for
// component identity so the rate limiter can key by component, and records
// the call as a replayOp so it still reaches the terminal handler for
// rendering.
func (s *samplingInjector) WithAttrs(attrs []slog.Attr) slog.Handler {
	ns := s.clone()
	ns.comp = sniffComponent(s.comp, attrs)
	ns.ops = append(ns.ops, replayOp{attrs: attrs})
	return ns
}

// WithGroup returns a new handler with name pushed as an open group,
// recording the call as a replayOp so it still reaches the terminal handler
// for rendering.
func (s *samplingInjector) WithGroup(name string) slog.Handler {
	if name == "" {
		return s
	}
	ns := s.clone()
	ns.ops = append(ns.ops, replayOp{group: name})
	return ns
}

// Enabled delegates to the current root handler.
func (s *samplingInjector) Enabled(ctx context.Context, l slog.Level) bool {
	return s.holder.Load().h.Enabled(ctx, l)
}

// Handle routes empty-message records directly to the bare terminal handler
// (bypassing the rate limiter, since blank-message records from different
// call sites would otherwise share a signature), and all other records
// through the current, possibly rate-limited, root handler with the
// component identity injected into ctx for the Matcher to read.
func (s *samplingInjector) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "" {
		// Bypass sampler: unrelated no-msg events must not collapse into one signature.
		bh := s.bareCache.Load()
		if bh == nil {
			h := replay(s.bare, s.ops)
			bh = &h
			s.bareCache.Store(bh)
		}
		return (*bh).Handle(ctx, r)
	}
	vh := s.holder.Load()
	c := s.cache.Load()
	if c == nil || c.version != vh.version {
		c = &cachedHandler{version: vh.version, h: replay(vh.h, s.ops)}
		s.cache.Store(c)
	}
	return c.h.Handle(withComponent(ctx, s.comp), r)
}

var _ slog.Handler = (*samplingInjector)(nil)
