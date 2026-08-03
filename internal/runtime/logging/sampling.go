package logging

import (
	"context"
	"log/slog"

	"go.uber.org/atomic"

	"github.com/prometheus/client_golang/prometheus"
	slogsampling "github.com/samber/slog-sampling"
	"github.com/samber/slog-sampling/buffer"

	"github.com/grafana/alloy/internal/util/metricsutil"
)

type componentInfo struct{ id, path string }

// sniffComponent reads component_id, controller_id, component_path, and
// controller_path from attrs and merges them onto base. It returns the
// result. If component_id is set, it wins over controller_id; the same
// precedence applies to component_path over controller_path.
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
		case "controller_path":
			if c.path == "" {
				c.path = a.Value.String()
			}
		}
	}
	return c
}

type ctxKey struct{}

// withComponent stores c in ctx. A Matcher can read it back later. This
// avoids passing c through every log call site.
func withComponent(ctx context.Context, c componentInfo) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// componentFromCtx returns the componentInfo stored by withComponent. It
// returns the zero value if none was stored.
func componentFromCtx(ctx context.Context) componentInfo {
	c, _ := ctx.Value(ctxKey{}).(componentInfo)
	return c
}

// compMatcher is the slog-sampling Matcher used to build the rate limiter's
// signature key. Two records share one signature, and so share one
// rate-limit budget, when they have the same component path, component ID,
// level, and message.
//
// component_path alone is not enough. It is the parent path (for example,
// "/" for every top-level component), so many components share it. Without
// component_id, different top-level components that log the same message at
// the same level would share one signature and suppress each other.
func compMatcher(ctx context.Context, r *slog.Record) string {
	c := componentFromCtx(ctx)
	return c.path + "\x00" + c.id + "\x00" + r.Level.String() + "\x00" + r.Message
}

// levelString converts a slog.Level to the lowercase label used in the
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

// rateLimitMetrics counts log lines dropped by the rate limiter, by level
// and component. A nil *rateLimitMetrics is valid: its methods do nothing.
// Callers do not need to check for a missing registerer.
type rateLimitMetrics struct {
	suppressed *prometheus.CounterVec
}

// newRateLimitMetrics registers the alloy_logging_suppressed_lines_total
// counter vector on reg. It returns nil if reg is nil, so callers that do
// not want metrics, such as tests, can safely pass a nil registerer.
func newRateLimitMetrics(reg prometheus.Registerer) *rateLimitMetrics {
	if reg == nil {
		return nil
	}
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "alloy_logging_suppressed_lines_total",
		Help: "Total log lines dropped by the logger's rate limiter, by level and component.",
	}, []string{"level", "component_id"})
	if existing := metricsutil.MustRegisterOrReturnExisting(reg, cv); existing != nil {
		cvExisting, ok := existing.(*prometheus.CounterVec)
		if !ok {
			panic("alloy_logging_suppressed_lines_total already registered with unexpected collector type")
		}
		cv = cvExisting
	}
	return &rateLimitMetrics{suppressed: cv}
}

// onDropped is the OnDropped hook for slog-sampling. It increments the
// suppressed-lines counter for the record's level and component.
func (m *rateLimitMetrics) onDropped(ctx context.Context, r slog.Record) {
	if m == nil {
		return
	}
	m.suppressed.WithLabelValues(levelString(r.Level), componentFromCtx(ctx).id).Inc()
}

// buildRoot wraps terminal with sampling when rate limiting is enabled.
// Otherwise it returns terminal unchanged.
//
// metrics is a live pointer, not a captured value: OnDropped reads
// metrics.Load() on every drop, instead of closing over one
// *rateLimitMetrics snapshot at build time. This lets InitRateLimitMetrics
// take effect even when it runs after the root handler was already built by
// an earlier Update call.
func buildRoot(o RateLimitingOptions, terminal slog.Handler, metrics *atomic.Pointer[rateLimitMetrics]) slog.Handler {
	if !o.Enabled {
		return terminal
	}
	opt := slogsampling.ThresholdSamplingOption{
		Tick:      o.Tick,
		Threshold: o.Threshold,
		Rate:      o.Rate,
		Matcher:   compMatcher,
		Buffer:    buffer.NewLRUBuffer[string](o.MaxSignatures),
		OnDropped: func(ctx context.Context, r slog.Record) {
			if metrics != nil {
				if m := metrics.Load(); m != nil {
					m.onDropped(ctx, r)
				}
			}
		},
		IncludeDroppedCount: true,
	}
	return opt.NewMiddleware()(terminal)
}

// replayOp records one WithAttrs or WithGroup call, so it can be replayed
// later on a new terminal handler. Only one of attrs or group is set.
type replayOp struct {
	attrs []slog.Attr
	group string
}

// versionedHandler pairs a handler with a version number. Logger's rlHolder
// uses it to hold the current root handler; Update increases the version
// each time the rate-limiting config changes. samplingInjector's cache uses
// the same type to hold its replay of that root handler, keyed by the same
// version, so it can compare versions to know when the replay is stale and
// must be rebuilt.
type versionedHandler struct {
	version uint64
	h       slog.Handler
}

// handlerBox wraps a slog.Handler so bareCache can store it in an
// atomic.Pointer. atomic.Pointer needs a concrete type, and slog.Handler is
// an interface, so the box supplies that concrete type.
type handlerBox struct {
	h slog.Handler
}

// samplingInjector is a slog.Handler between component loggers and the
// shared root handler, which may be rate-limited. It does three things:
//
//   - It tracks component identity (comp) read from WithAttrs calls. The
//     rate limiter's Matcher reads comp from the context to key by
//     component.
//   - It records every WithAttrs/WithGroup call as a replayOp. When the root
//     handler changes (for example, a rate-limiting config change) or is
//     bypassed (an empty-message record), it replays these calls on the new
//     or bare terminal handler. This keeps the rendered output the same.
//   - It bypasses the rate limiter for empty-message records. Without this,
//     all empty-message records would share one signature.
type samplingInjector struct {
	comp componentInfo
	ops  []replayOp

	holder *atomic.Pointer[versionedHandler]
	bare   slog.Handler // bare terminal handler (no per-component attrs), used to bypass sampling for empty-message records

	// bgCtx is context.Background() with comp already added by
	// withComponent. It is computed once per injector, not on every Handle
	// call, because slog.Logger.Info/Warn/Error always pass
	// context.Background(). This is by far the most common case on the
	// admit path; see Handle.
	bgCtx context.Context

	cache     atomic.Pointer[versionedHandler]
	bareCache atomic.Pointer[handlerBox]
}

// newSamplingInjector creates a samplingInjector. holder points to the
// current root handler, which may be wrapped for sampling. bare is the
// terminal handler used to bypass sampling for empty-message records.
func newSamplingInjector(holder *atomic.Pointer[versionedHandler], bare slog.Handler) *samplingInjector {
	s := &samplingInjector{holder: holder, bare: bare}
	s.bgCtx = withComponent(context.Background(), s.comp)
	return s
}

// replay applies a recorded sequence of WithAttrs/WithGroup calls to h, in
// order. The result is the same handler as calling them directly on h.
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

// clone returns a new samplingInjector. It shares this injector's holder and
// bare terminal, but gets its own copy of ops and fresh, empty caches: a new
// ops slice means any old cached replay is stale.
func (s *samplingInjector) clone() *samplingInjector {
	// The new injector shares holder and bare, and starts with empty caches
	// because ops will differ. clone itself does not change comp (WithAttrs
	// changes it on the returned injector below), so bgCtx also carries over
	// unchanged from the parent.
	ns := &samplingInjector{comp: s.comp, holder: s.holder, bare: s.bare, bgCtx: s.bgCtx}
	ns.ops = make([]replayOp, len(s.ops), len(s.ops)+1)
	copy(ns.ops, s.ops)
	return ns
}

// WithAttrs returns a new handler with attrs bound. It also reads attrs for
// component identity, so the rate limiter can key by component, and records
// the call as a replayOp, so the terminal handler still renders it.
func (s *samplingInjector) WithAttrs(attrs []slog.Attr) slog.Handler {
	ns := s.clone()
	ns.comp = sniffComponent(s.comp, attrs)
	ns.bgCtx = withComponent(context.Background(), ns.comp)
	ns.ops = append(ns.ops, replayOp{attrs: attrs})
	return ns
}

// WithGroup returns a new handler with name added as an open group. It
// records the call as a replayOp, so the terminal handler still renders it.
func (s *samplingInjector) WithGroup(name string) slog.Handler {
	if name == "" {
		return s
	}
	ns := s.clone()
	ns.ops = append(ns.ops, replayOp{group: name})
	return ns
}

// Enabled calls the bare terminal handler, not the root handler, which may
// be wrapped for sampling. Sampling only drops records in Handle; it never
// changes whether a level is enabled. Calling the bare handler here skips
// the sampling wrapper's per-call cost on every slog.Info/Warn/Error call.
// s.bare uses the shared LevelVar that Update changes, and level gating does
// not depend on component, so this is correct for every derived injector,
// not just the root one.
func (s *samplingInjector) Enabled(ctx context.Context, l slog.Level) bool {
	return s.bare.Enabled(ctx, l)
}

// Handle sends empty-message records straight to the bare terminal handler.
// This skips the rate limiter, because blank-message records from different
// call sites would otherwise share one signature. All other records go
// through the current root handler, which may be rate-limited. Handle adds
// the component identity to ctx so the Matcher can read it.
func (s *samplingInjector) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "" {
		// Skip the sampler: unrelated no-message records must not share one signature.
		bh := s.bareCache.Load()
		if bh == nil {
			bh = &handlerBox{h: replay(s.bare, s.ops)}
			s.bareCache.Store(bh)
		}
		return bh.h.Handle(ctx, r)
	}
	vh := s.holder.Load()
	c := s.cache.Load()
	if c == nil || c.version != vh.version {
		c = &versionedHandler{version: vh.version, h: replay(vh.h, s.ops)}
		s.cache.Store(c)
	}
	// slog.Logger.Info/Warn/Error always pass context.Background(). Reuse
	// the precomputed component ctx for this common case, instead of an
	// allocation from context.WithValue on every admitted line. For any
	// other ctx, for example from InfoContext, inject fresh instead: it
	// may carry values or a Done channel that must not be lost.
	var cctx context.Context
	if ctx == context.Background() {
		cctx = s.bgCtx
	} else {
		cctx = withComponent(ctx, s.comp)
	}
	return c.h.Handle(cctx, r)
}

var _ slog.Handler = (*samplingInjector)(nil)
