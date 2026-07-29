package logging

import (
	"context"
	"log/slog"

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
// path, level, and message.
func compMatcher(ctx context.Context, r *slog.Record) string {
	c := componentFromCtx(ctx)
	return c.path + "\x00" + r.Level.String() + "\x00" + r.Message
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
