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

// NewCRI creates a predefined pipeline for parsing entries in the CRI log
// format.
func NewCRI(logger log.Logger, config CRIConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	base := []StageConfig{
		{
			RegexConfig: &RegexConfig{
				Expression: "^(?s)(?P<time>\\S+?) (?P<stream>stdout|stderr) (?P<flags>\\S+?) (?P<content>.*)$",
			},
		},
		{
			LabelsConfig: &LabelsConfig{
				Values: map[string]*string{"stream": nil},
			},
		},
		{
			TimestampConfig: &TimestampConfig{
				Source: "time",
				Format: RFC3339Nano,
			},
		},
		{
			OutputConfig: &OutputConfig{
				Source: "content",
			},
		},
	}

	p, err := NewPipeline(logger, base, nil, registerer, minStability)
	if err != nil {
		return nil, err
	}

	c := cri{
		cfg:  config,
		base: p,
	}
	c.partialLines = make(map[model.Fingerprint]Entry, c.cfg.MaxPartialLines)
	return &c, nil
}

type cri struct {
	// bounded buffer for CRI-O Partial logs lines (identified with tag `P` till we reach first `F`)
	partialLines map[model.Fingerprint]Entry
	cfg          CRIConfig
	base         *Pipeline
}

var _ Stage = (*cri)(nil)

// Name implement the Stage interface.
func (c *cri) Name() string {
	return StageTypeCRI
}

// Cleanup implements Stage.
func (*cri) Cleanup() {
	// no-op
}

// implements Stage interface
func (c *cri) Run(entry chan Entry) chan Entry {
	entry = c.base.Run(entry)

	in := RunWithSkipOrSendMany(entry, func(e Entry) ([]Entry, bool) {
		fingerprint := e.Labels.Fingerprint()

		// We received partial-line (tag: "P")
		if e.Extracted["flags"] == "P" {
			if len(c.partialLines) >= c.cfg.MaxPartialLines {
				// Merge existing partialLines
				entries := make([]Entry, 0, len(c.partialLines))
				for _, v := range c.partialLines {
					entries = append(entries, v)
				}

				level.Warn(c.base.logger).Log("msg", "cri stage: partial lines upperbound exceeded. merging it to single line", "threshold", c.cfg.MaxPartialLines)

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

			return []Entry{e}, true // it's a partial-line so skip it.
		}

		// Now we got full-line (tag: "F").
		// 1. If any old partialLines matches with this full-line stream, merge it
		// 2. Else just return the full line.
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

	return in
}

func (c *cri) ensureTruncateIfRequired(e *Entry) {
	if c.cfg.MaxPartialLineSizeTruncate && len(e.Line) > int(c.cfg.MaxPartialLineSize) {
		e.Line = e.Line[:c.cfg.MaxPartialLineSize]
	}
}

func NewCRI2(logger log.Logger, cfg CRIConfig, _ prometheus.Registerer, _ featuregate.Stability) (Stage, error) {
	return &cri2{
		logger:       logger,
		cfg:          cfg,
		partialLines: make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
	}, nil
}

var _ Stage = (*cri2)(nil)

type cri2 struct {
	logger       log.Logger
	cfg          CRIConfig
	partialLines map[model.Fingerprint]Entry
}

func (c *cri2) Name() string { return StageTypeCRI }

func (c *cri2) Run(in chan Entry) chan Entry {
	return RunWithSkipOrSendMany(in, func(e Entry) ([]Entry, bool) {
		parsed, ok := crip.ParseCRI([]byte(e.Line))
		if !ok {
			return []Entry{e}, false
		}

		e.Extracted["content"] = parsed.Content
		e.Extracted["time"] = parsed.Timestamp
		e.Extracted["flags"] = parsed.Flag.String()
		e.Extracted["stream"] = parsed.Stream.String()

		e.Line = parsed.Content
		// FIXME, there is some non obvious behaviour when using timestamp stage..
		ts, err := time.Parse(time.RFC3339Nano, parsed.Timestamp)
		if err == nil {
			e.Timestamp = ts
		}

		e.Labels["stream"] = model.LabelValue(parsed.Stream.String())

		fingerprint := e.Labels.Fingerprint()
		// We received partial-line (tag: "P")
		if parsed.Flag == crip.FlagPartial {
			if len(c.partialLines) >= c.cfg.MaxPartialLines {
				level.Warn(c.logger).Log("msg", "cri stage: partial lines upperbound exceeded. merging it to single line", "threshold", c.cfg.MaxPartialLines)

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

func (c *cri2) ensureTruncateIfRequired(e *Entry) {
	if c.cfg.MaxPartialLineSizeTruncate && len(e.Line) > int(c.cfg.MaxPartialLineSize) {
		e.Line = e.Line[:c.cfg.MaxPartialLineSize]
	}
}

func (c *cri2) Cleanup() {}
