package stages

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	crip "github.com/grafana/alloy/internal/component/loki/process/stages/cri"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax"
)

// CRIConfig is an empty struct that is used to enable a pre-defined pipeline
// for decoding entries that are using the CRI logging format.
type CRIConfig struct {
	MaxPartialLines            int    `alloy:"max_partial_lines,attr,optional"`
	MaxPartialLineSize         uint64 `alloy:"max_partial_line_size,attr,optional"`
	MaxPartialLineSizeTruncate bool   `alloy:"max_partial_line_size_truncate,attr,optional"`
}

var (
	_ syntax.Defaulter = (*CRIConfig)(nil)
	_ syntax.Validator = (*CRIConfig)(nil)
)

// DefaultCRIConfig contains the default CRIConfig values.
var DefaultCRIConfig = CRIConfig{
	MaxPartialLines:            100,
	MaxPartialLineSize:         0,
	MaxPartialLineSizeTruncate: false,
}

// SetToDefault implements syntax.Defaulter.
func (args *CRIConfig) SetToDefault() {
	*args = DefaultCRIConfig
}

// Validate implements syntax.Validator.
func (args *CRIConfig) Validate() error {
	if args.MaxPartialLines <= 0 {
		return fmt.Errorf("max_partial_lines must be greater than 0")
	}

	return nil
}

func NewCRI(logger log.Logger, cfg CRIConfig, registerer prometheus.Registerer, _ featuregate.Stability) (Stage, error) {
	partialLinesFlushedMetric, err := getPartialLinesFlushedMetric(registerer)
	if err != nil {
		level.Error(logger).Log("msg", "failed to register cri partial lines flushed metric", "err", err)
	}
	linesTruncatedMetric, err := getLinesTruncatedMetric(registerer)
	if err != nil {
		level.Error(logger).Log("msg", "failed to register cri lines truncated metric", "err", err)
	}
	return &cri{
		logger:                    logger,
		cfg:                       cfg,
		partialLines:              make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
		partialLinesFlushedMetric: partialLinesFlushedMetric,
		linesTruncatedMetric:      linesTruncatedMetric,
	}, nil
}

func getPartialLinesFlushedMetric(registerer prometheus.Registerer) (prometheus.Counter, error) {
	metric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_process_cri_partial_lines_flushed_total",
		Help: "A count of partial lines that were flushed prematurely due to the max_partial_lines limit being exceeded",
	})
	err := registerer.Register(metric)
	if err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return existing.ExistingCollector.(prometheus.Counter), nil
		}
		return nil, err
	}
	return metric, nil
}

func getLinesTruncatedMetric(registerer prometheus.Registerer) (prometheus.Counter, error) {
	metric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_process_cri_lines_truncated_total",
		Help: "A count of lines that were truncated due to the max_partial_line_size limit",
	})
	err := registerer.Register(metric)
	if err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return existing.ExistingCollector.(prometheus.Counter), nil
		}
		return nil, err
	}
	return metric, nil
}

var _ Stage = (*cri)(nil)

type cri struct {
	logger                    log.Logger
	cfg                       CRIConfig
	partialLines              map[model.Fingerprint]Entry
	partialLinesFlushedMetric prometheus.Counter
	linesTruncatedMetric      prometheus.Counter
}

func (c *cri) Name() string { return StageTypeCRI }

const (
	criTime    = "time"
	criFlags   = "flags"
	criStream  = "stream"
	criContent = "content"
)

func (c *cri) Run(in chan Entry) chan Entry {
	return RunWithSkipOrSendMany(in, func(e Entry) ([]Entry, bool) {
		parsed, ok := crip.ParseCRI(e.Line)
		if !ok {
			return []Entry{e}, false
		}

		// NOTE: Previous implementation used a "sub-pipeline"
		// to parse CRI logs where the regex stage added these fields
		// as "extracted" values so the other stages could operate on them.
		// We don't need this anymore but it would be a breaking change to
		// no longer set these.
		e.Extracted[criTime] = parsed.Timestamp
		e.Extracted[criFlags] = parsed.Flag.String()
		e.Extracted[criStream] = parsed.Stream.String()
		e.Extracted[criContent] = parsed.Content

		e.Line = parsed.Content

		ts, err := time.Parse(time.RFC3339Nano, parsed.Timestamp)
		if err == nil {
			e.Timestamp = ts
		}

		e.Labels[criStream] = model.LabelValue(parsed.Stream.String())

		fingerprint := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			if len(c.partialLines) >= c.cfg.MaxPartialLines {
				level.Warn(c.logger).Log("msg", "cri stage: partial lines upperbound exceeded. merging it to single line", "threshold", c.cfg.MaxPartialLines)
				if c.partialLinesFlushedMetric != nil {
					c.partialLinesFlushedMetric.Add(float64(len(c.partialLines)))
				}

				// Merge existing partialLines
				entries := make([]Entry, 0, len(c.partialLines))
				for _, v := range c.partialLines {
					entries = append(entries, v)
				}

				c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
				c.ensureTruncateIfRequired(&e)
				c.partialLines[fingerprint] = e

				return entries, false
			}

			prev, ok := c.partialLines[fingerprint]
			if ok {
				var builder strings.Builder
				builder.WriteString(prev.Line)
				builder.WriteString(e.Line)
				e.Line = builder.String()
			}
			c.ensureTruncateIfRequired(&e)
			c.partialLines[fingerprint] = e

			// it's a partial-line so skip it.
			return nil, true
		}

		// We got full-line 'F'.
		// If any old partial lines matches with this full-line stream, merge it,
		// else just return the full line.
		prev, ok := c.partialLines[fingerprint]
		if ok {
			var builder strings.Builder
			builder.WriteString(prev.Line)
			builder.WriteString(e.Line)
			e.Line = builder.String()
			c.ensureTruncateIfRequired(&e)
			delete(c.partialLines, fingerprint)
		}

		return []Entry{e}, false
	})
}

func (c *cri) ensureTruncateIfRequired(e *Entry) {
	if c.cfg.MaxPartialLineSizeTruncate && len(e.Line) > int(c.cfg.MaxPartialLineSize) {
		e.Line = e.Line[:c.cfg.MaxPartialLineSize]
		if c.linesTruncatedMetric != nil {
			c.linesTruncatedMetric.Inc()
		}
	}
}

func (c *cri) Cleanup() {}
