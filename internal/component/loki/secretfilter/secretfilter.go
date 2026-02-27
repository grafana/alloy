package secretfilter

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
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
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	OriginLabel    string              `alloy:"origin_label,attr,optional"`    // The label name to use for tracking metrics by origin (if empty, no origin metrics are collected)
	RedactWith     string              `alloy:"redact_with,attr,optional"`     // Template for redaction placeholder; $SECRET_NAME and $SECRET_HASH are replaced. When set, percentage-based redaction is not used.
	RedactPercent  uint                `alloy:"redact_percent,attr,optional"`  // When redact_with is not set: percent of the secret to redact (1-100; gitleaks-style: show leading (100-N)% + "...", 100 = "REDACTED"). 0 or unset defaults to 80.
	GitleaksConfig string              `alloy:"gitleaks_config,attr,optional"` // Path to a gitleaks TOML config file; if empty, the default gitleaks config is used
	Rate           float64             `alloy:"rate,attr,optional"`            // Sampling rate in [0.0, 1.0]: fraction of entries to process through the secret filter; rest are forwarded unchanged. 1.0 = process all (default).
}

// Exports holds the values exported by the loki.secretfilter component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// defaultRate is the default sampling rate (1.0 = process all entries).
const defaultRate = 1.0

// defaultRedactPercent is the percentage used for gitleaks-style redaction when redact_with is not set and redact_percent is 0 or out of range.
const defaultRedactPercent uint = 80

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{
	Rate:          defaultRate,
	RedactPercent: defaultRedactPercent,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// ErrInvalidRate is the error returned when rate is not in [0, 1].
const ErrInvalidRate = "secretfilter rate must be between 0.0 and 1.0, received %f"

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Rate < 0.0 || args.Rate > 1.0 {
		return fmt.Errorf(ErrInvalidRate, args.Rate)
	}
	return nil
}

var _ syntax.Validator = (*Arguments)(nil)

// maxRandomNumber is the maximum value used for sampling boundary
const maxRandomNumber = ^(uint64(1) << 63) // 0x7fffffffffffffff

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

	// sampling state (used when 0 < Rate < 1)
	samplingBoundary uint64
	samplingSource   rand.Source

	metrics            *metrics
	debugDataPublisher livedebugging.DebugDataPublisher
}

// Metrics exposed by this component:
//
//   - loki_secretfilter_secrets_redacted_total: Total number of secrets that have been redacted.
//   - loki_secretfilter_secrets_redacted_by_rule_total: Number of secrets redacted, partitioned by rule name.
//   - loki_secretfilter_secrets_redacted_by_origin: Number of secrets redacted, partitioned by origin label value (only registered when origin_label is set).
//   - loki_secretfilter_processing_duration_seconds: Summary of time taken to process and redact log entries.
//   - loki_secretfilter_entries_bypassed_total: Total number of entries forwarded without processing due to sampling.
type metrics struct {
	// Total number of secrets redacted
	secretsRedactedTotal prometheus.Counter

	// Number of secrets redacted by rule type
	secretsRedactedByRule *prometheus.CounterVec

	// Number of secrets redacted by specified labels
	secretsRedactedByOrigin *prometheus.CounterVec

	// Summary of time taken for redaction log processing
	processingDuration prometheus.Summary

	// Total number of entries bypassed by sampling (forwarded unchanged)
	entriesBypassedTotal prometheus.Counter
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

	m.entriesBypassedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "entries_bypassed_total",
		Help:      "Total number of entries forwarded without processing due to sampling.",
	})

	if reg != nil {
		m.secretsRedactedTotal = util.MustRegisterOrGet(reg, m.secretsRedactedTotal).(prometheus.Counter)
		m.secretsRedactedByRule = util.MustRegisterOrGet(reg, m.secretsRedactedByRule).(*prometheus.CounterVec)
		if originLabel != "" {
			m.secretsRedactedByOrigin = util.MustRegisterOrGet(reg, m.secretsRedactedByOrigin).(*prometheus.CounterVec)
		}
		m.processingDuration = util.MustRegisterOrGet(reg, m.processingDuration).(prometheus.Summary)
		m.entriesBypassedTotal = util.MustRegisterOrGet(reg, m.entriesBypassedTotal).(prometheus.Counter)
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
			var newEntry loki.Entry
			if c.shouldProcessEntry() {
				newEntry = c.processEntry(entry)
				c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
					componentID,
					livedebugging.LokiLog,
					1,
					func() string {
						return fmt.Sprintf("%s => %s", entry.Line, newEntry.Line)
					},
				))
			} else {
				newEntry = entry
				c.metrics.entriesBypassedTotal.Inc()
			}

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

// shouldProcessEntry returns true if this entry should be processed through the secret filter (rate = probability of "keep" / process).
func (c *Component) shouldProcessEntry() bool {
	rate := c.args.Rate
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}
	return c.samplingBoundary >= c.samplingRandomID()&maxRandomNumber
}

// samplingRandomID returns a random uint64 in [1, maxRandomNumber] for sampling.
// If samplingSource is nil (e.g. rate was 0 or 1), returns maxRandomNumber so the caller does not panic.
func (c *Component) samplingRandomID() uint64 {
	if c.samplingSource == nil {
		return maxRandomNumber
	}
	val := uint64(c.samplingSource.Int63())
	for val == 0 {
		val = uint64(c.samplingSource.Int63())
	}
	return val
}

func (c *Component) processEntry(entry loki.Entry) loki.Entry {
	start := time.Now()
	defer func() {
		c.metrics.processingDuration.Observe(time.Since(start).Seconds())
	}()

	// Scan the log line for secrets
	findings := c.detector.DetectString(entry.Line)

	// If no secrets found, return the original entry
	if len(findings) == 0 {
		return entry
	}

	return c.redactLine(entry, findings)
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
	if newArgs.Rate > 0 && newArgs.Rate < 1 {
		c.samplingBoundary = uint64(float64(maxRandomNumber) * math.Max(0, math.Min(newArgs.Rate, 1)))
		c.samplingSource = rand.NewSource(time.Now().UnixNano())
	} else {
		c.samplingBoundary = 0
		c.samplingSource = nil
	}
	c.metrics = newMetrics(c.opts.Registerer, newArgs.OriginLabel)

	return nil
}

func (c *Component) LiveDebugging() {}
