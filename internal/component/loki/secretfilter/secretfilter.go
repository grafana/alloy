package secretfilter

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/spf13/viper"
	"github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.secretfilter",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the secretfilter component.
type Arguments struct {
	ForwardTo         []loki.LogsReceiver `alloy:"forward_to,attr"`
	OriginLabel       string              `alloy:"origin_label,attr,optional"`       // The label name to use for tracking metrics by origin (if empty, no origin metrics are collected)
	RedactWith        string              `alloy:"redact_with,attr,optional"`        // Template for redaction placeholder; $SECRET_NAME and $SECRET_HASH are replaced. When set, percentage-based redaction is not used.
	RedactPercent     uint                `alloy:"redact_percent,attr,optional"`     // When redact_with is not set: percent of the secret to redact (1-100; gitleaks-style: show leading (100-N)% + "...", 100 = "REDACTED"). 0 or unset defaults to 80.
	GitleaksConfig    string              `alloy:"gitleaks_config,attr,optional"`    // Path to a gitleaks TOML config file; if empty, the default gitleaks config is used
	ProcessingTimeout time.Duration       `alloy:"processing_timeout,attr,optional"` // Maximum time allowed to process a single log entry. 0 (default) disables the timeout.
	DropOnTimeout     bool                `alloy:"drop_on_timeout,attr,optional"`    // When true, entries that exceed processing_timeout are dropped instead of forwarded unredacted. Requires processing_timeout to be set.
}

// Exports holds the values exported by the loki.secretfilter component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// defaultRedactPercent is the percentage used for gitleaks-style redaction when redact_with is not set and redact_percent is 0 or out of range.
const defaultRedactPercent uint = 80

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.secretfilter component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver
	fanout   []loki.LogsReceiver
	detector *detect.Detector

	// redactPercent is the effective percentage (1-100) for gitleaks-style redaction when redact_with is not set. Set at build/update.
	redactPercent uint

	metrics            *metrics
	debugDataPublisher livedebugging.DebugDataPublisher
}

// Metrics exposed by this component:
//
//   - loki_secretfilter_secrets_redacted_total: Total number of secrets that have been redacted.
//   - loki_secretfilter_secrets_redacted_by_rule_total: Number of secrets redacted, partitioned by rule name.
//   - loki_secretfilter_secrets_redacted_by_origin: Number of secrets redacted, partitioned by origin label value (only registered when origin_label is set).
//   - loki_secretfilter_processing_duration_seconds: Summary of time taken to process and redact log entries.
//   - loki_secretfilter_lines_timed_out_total: Total number of log lines that exceeded the processing timeout (regardless of whether they were dropped or forwarded).
//   - loki_secretfilter_lines_dropped_total: Total number of log lines dropped due to processing timeout (subset of lines_timed_out_total, only when drop_on_timeout is true).
type metrics struct {
	// Total number of secrets redacted
	secretsRedactedTotal prometheus.Counter

	// Number of secrets redacted by rule type
	secretsRedactedByRule *prometheus.CounterVec

	// Number of secrets redacted by specified labels
	secretsRedactedByOrigin *prometheus.CounterVec

	// Summary of time taken for redaction log processing
	processingDuration prometheus.Summary

	// Total number of log lines that exceeded the processing timeout, regardless of whether they were dropped or forwarded unredacted
	linesTimedOutTotal prometheus.Counter

	// Total number of log lines dropped due to processing timeout
	linesDroppedTotal prometheus.Counter
}

// newMetrics creates a new set of metrics for the secretfilter component.
func newMetrics(reg prometheus.Registerer, originLabel string) *metrics {
	var m metrics

	m.secretsRedactedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_redacted_total",
		Help:      "Total number of secrets that have been redacted.",
	})

	m.secretsRedactedByRule = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_redacted_by_rule_total",
		Help:      "Number of secrets redacted, partitioned by rule name.",
	}, []string{"rule"})

	if originLabel != "" {
		m.secretsRedactedByOrigin = prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_secretfilter",
			Name:      "secrets_redacted_by_origin",
			Help:      "Number of secrets redacted, partitioned by origin label value.",
		}, []string{"origin"})
	}

	m.processingDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: "loki_secretfilter",
		Name:      "processing_duration_seconds",
		Help:      "Summary of the time taken to process and redact logs in seconds.",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.99: 0.001,
		},
	})

	m.linesTimedOutTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "lines_timed_out_total",
		Help:      "Total number of log lines that exceeded the processing timeout, regardless of whether they were dropped or forwarded unredacted.",
	})

	m.linesDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "lines_dropped_total",
		Help:      "Total number of log lines dropped due to processing timeout.",
	})

	if reg != nil {
		m.secretsRedactedTotal = util.MustRegisterOrGet(reg, m.secretsRedactedTotal).(prometheus.Counter)
		m.secretsRedactedByRule = util.MustRegisterOrGet(reg, m.secretsRedactedByRule).(*prometheus.CounterVec)
		if originLabel != "" {
			m.secretsRedactedByOrigin = util.MustRegisterOrGet(reg, m.secretsRedactedByOrigin).(*prometheus.CounterVec)
		}
		m.processingDuration = util.MustRegisterOrGet(reg, m.processingDuration).(prometheus.Summary)
		m.linesTimedOutTotal = util.MustRegisterOrGet(reg, m.linesTimedOutTotal).(prometheus.Counter)
		m.linesDroppedTotal = util.MustRegisterOrGet(reg, m.linesDroppedTotal).(prometheus.Counter)
	}

	return &m
}

// loadGitleaksConfig reads a gitleaks TOML config from path and returns a config.Config.
func loadGitleaksConfig(path string) (config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("read gitleaks config: %w", err)
	}
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return config.Config{}, fmt.Errorf("parse gitleaks config: %w", err)
	}
	var vc config.ViperConfig
	if err := v.Unmarshal(&vc); err != nil {
		return config.Config{}, fmt.Errorf("unmarshal gitleaks config: %w", err)
	}
	cfg, err := vc.Translate()
	if err != nil {
		return config.Config{}, fmt.Errorf("translate gitleaks config: %w", err)
	}
	cfg.Path = path
	return cfg, nil
}

// newDetectorFromArgs creates a gitleaks detector from component arguments.
// If GitleaksConfig is empty, the default gitleaks config is used.
func newDetectorFromArgs(args Arguments) (*detect.Detector, error) {
	if args.GitleaksConfig == "" {
		return detect.NewDetectorDefaultConfig()
	}
	cfg, err := loadGitleaksConfig(args.GitleaksConfig)
	if err != nil {
		return nil, err
	}
	return detect.NewDetector(cfg), nil
}

// New creates a new loki.secretfilter component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	detector, err := newDetectorFromArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitleaks detector: %w", err)
	}

	c := &Component{
		opts:               o,
		receiver:           loki.NewLogsReceiver(loki.WithComponentID(o.ID)),
		detector:           detector,
		metrics:            newMetrics(o.Registerer, args.OriginLabel),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Call to Update() once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	componentID := livedebugging.ComponentID(c.opts.ID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			// Start processing the log entry to redact secrets
			newEntry, dropped := c.processEntry(ctx, entry)
			if dropped {
				c.mut.RUnlock()
				continue
			}

			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.LokiLog,
				1,
				func() string {
					return fmt.Sprintf("%s => %s", entry.Line, newEntry.Line)
				},
			))

			for _, f := range c.fanout {
				select {
				case <-ctx.Done():
					c.mut.RUnlock()
					return nil
				case f.Chan() <- newEntry:
				}
			}
			c.mut.RUnlock()
		}
	}
}

// processEntry scans the log entry for secrets and redacts them. Returns the
// processed entry and a boolean indicating whether the entry should be dropped.
// If processing_timeout is exceeded and drop_on_timeout is false (default),
// the original unredacted entry is forwarded. If processing_timeout is
// exceeded and drop_on_timeout is true, the entry is dropped.
func (c *Component) processEntry(ctx context.Context, entry loki.Entry) (loki.Entry, bool) {
	start := time.Now()
	defer func() {
		c.metrics.processingDuration.Observe(time.Since(start).Seconds())
	}()

	if timeout := c.args.ProcessingTimeout; timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	findings := c.detector.DetectContext(ctx, detect.Fragment{Raw: entry.Line})

	if ctx.Err() != nil {
		c.metrics.linesTimedOutTotal.Inc()
		if c.args.DropOnTimeout {
			c.metrics.linesDroppedTotal.Inc()
			return loki.Entry{}, true
		}
		return entry, false
	}

	if len(findings) == 0 {
		return entry, false
	}
	return c.redactLine(entry, findings), false
}

// redactLine redacts each finding in the log line and records metrics.
func (c *Component) redactLine(entry loki.Entry, findings []report.Finding) loki.Entry {
	line := entry.Line
	for i := range findings {
		finding := &findings[i]
		ruleName := finding.RuleID
		originalSecret := finding.Secret

		var replacement string
		if c.args.RedactWith != "" {
			redactWith := c.args.RedactWith
			redactWith = strings.ReplaceAll(redactWith, "$SECRET_NAME", ruleName)
			redactWith = strings.ReplaceAll(redactWith, "$SECRET_HASH", hashSecret(originalSecret))
			replacement = redactWith
		} else {
			// Percentage-based redaction (default is 80%)
			finding.Redact(c.redactPercent)
			replacement = finding.Secret
		}

		line = strings.ReplaceAll(line, originalSecret, replacement)

		c.metrics.secretsRedactedTotal.Inc()
		c.metrics.secretsRedactedByRule.WithLabelValues(ruleName).Inc()
		if c.args.OriginLabel != "" && len(entry.Labels) > 0 {
			if value, ok := entry.Labels[model.LabelName(c.args.OriginLabel)]; ok {
				c.metrics.secretsRedactedByOrigin.WithLabelValues(string(value)).Inc()
			}
		}
	}
	entry.Line = line
	return entry
}

// hashSecret returns a short hex-encoded SHA1 hash of the secret for use in redaction placeholders.
func hashSecret(secret string) string {
	hasher := sha1.New()
	hasher.Write([]byte(secret))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	detector, err := newDetectorFromArgs(newArgs)
	if err != nil {
		return fmt.Errorf("failed to create gitleaks detector: %w", err)
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = newArgs
	c.fanout = newArgs.ForwardTo
	c.detector = detector
	if newArgs.RedactPercent >= 1 && newArgs.RedactPercent <= 100 {
		c.redactPercent = newArgs.RedactPercent
	} else {
		c.redactPercent = defaultRedactPercent
	}
	c.metrics = newMetrics(c.opts.Registerer, newArgs.OriginLabel)

	return nil
}

func (c *Component) LiveDebugging() {}
